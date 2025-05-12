package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/fatih/color"
	"github.com/kr/text"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

var rootFlags struct {
	Debug     bool
	Directory string
}

var rootCmd = &cobra.Command{
	Use: "av",

	// Don't automatically print errors or usage information (we handle that ourselves).
	// Cobra still prints usage if you return cmd.Usage() from RunE.
	SilenceErrors: true,
	SilenceUsage:  true,

	// Don't show "completion" command in help menu
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},

	// Run setup before invoking any child commands.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if rootFlags.Debug {
			logrus.SetLevel(logrus.DebugLevel)
			logrus.WithField("av_version", config.Version).Debug("enabled debug logging")
		}

		repoConfigDir := ""
		repo, err := getRepo()
		// If we weren't able to load the Git repo, that probably just means the
		// command isn't being run from inside a repo. That's fine, we just
		// don't need to bother reading repo-local config.
		if err != nil {
			logrus.WithError(err).Debug("unable to load Git repo (probably not inside a repo)")
		} else {
			gitCommonDir, err := repo.Git("rev-parse", "--git-common-dir")
			if err != nil {
				logrus.WithError(err).Warning("failed to determine $GIT_COMMON_DIR")
			} else {
				gitCommonDir, err = filepath.Abs(gitCommonDir)
				if err != nil {
					logrus.WithError(err).Warning("failed to determine $GIT_COMMON_DIR")
				} else {
					logrus.WithField("git_common_dir", gitCommonDir).Debug("loaded Git repo")
					repoConfigDir = filepath.Join(gitCommonDir, "av")
				}
			}
		}

		// Note: this only returns an error if config exists and it can't be
		// read/parsed. It doesn't return an error if no config file exists.
		if err := config.Load(repoConfigDir); err != nil {
			return errors.Wrap(err, "failed to load configuration")
		}
		if err := config.LoadUserState(); err != nil {
			return errors.Wrap(err, "failed to load the user state")
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(
		&rootFlags.Debug, "debug", false,
		"enable verbose debug logging",
	)
	rootCmd.PersistentFlags().StringVarP(
		&rootFlags.Directory, "repo", "C", "",
		"directory to use for git repository",
	)
	rootCmd.AddCommand(
		adoptCmd,
		authCmd,
		branchCmd,
		branchMetaCmd,
		commitCmd,
		diffCmd,
		fetchCmd,
		initCmd,
		nextCmd,
		orphanCmd,
		prCmd,
		prevCmd,
		reorderCmd,
		reparentCmd,
		splitCommitCmd,
		stackCmd,
		switchCmd,
		syncCmd,
		restackCmd,
		tidyCmd,
		treeCmd,
		versionCmd,
	)
}

func main() {
	// Note: this doesn't include whatever time is spent in initializing the
	// runtime and various packages (e.g., package init functions).
	startTime := time.Now()
	colors.SetupBackgroundColorTypeFromEnv()
	err := rootCmd.Execute()
	logrus.WithField("duration", time.Since(startTime)).Debug("command exited")
	checkCliVersion()
	var exitSilently actions.ErrExitSilently
	if errors.As(err, &exitSilently) {
		os.Exit(exitSilently.ExitCode)
	}
	if err != nil {
		// In debug mode, show more detailed information about the error
		// (including the stack trace if using pkg/errors).
		if rootFlags.Debug {
			stackTrace := fmt.Sprintf("%+v", err)
			fmt.Fprintf(os.Stderr, "error: %s\n%s\n", err, text.Indent(stackTrace, "\t"))
		} else {
			fmt.Fprint(os.Stderr, renderError(err))
		}

		os.Exit(1)
	}
}

func checkCliVersion() {
	if config.Version == config.VersionDev {
		logrus.Debug("Skipping CLI version check (development version)")
		return
	}
	for _, arg := range os.Args {
		if arg == "completion" {
			// Skip the update check as it can slow down the shell initialization on a
			// slow network connection. This can have a false positive, but this is
			// anyway an optional check.
			logrus.Debug("Skipping CLI version check (shell completion)")
			return
		}
	}
	latest, err := config.FetchLatestVersion()
	if err != nil {
		logrus.WithError(err).Debug("failed to determine latest released version of av")
		return
	}
	logrus.WithField("latest", latest).Debug("fetched latest released version")
	if semver.Compare(config.Version, latest) < 0 {
		c := color.New(color.Faint, color.Bold)
		fmt.Fprint(
			os.Stderr,
			c.Sprint(">> A new version of av is available: "),
			color.RedString(config.Version),
			c.Sprint(" => "),
			color.GreenString(latest),
			"\n",
			c.Sprint(">> https://docs.aviator.co/reference/aviator-cli/installation#upgrade\n"),
		)
	}
}

var (
	once             sync.Once
	lazyGithubClient *gh.Client
)

func discoverGitHubAPIToken() string {
	if config.Av.GitHub.Token != "" {
		return config.Av.GitHub.Token
	}
	if ghCli, err := exec.LookPath("gh"); err == nil {
		var stdout bytes.Buffer
		cmd := exec.Command(ghCli, "auth", "token")
		cmd.Stdout = &stdout
		cmd.Stderr = nil
		if err := cmd.Run(); err == nil {
			return strings.TrimSpace(stdout.String())
		}
	}
	return ""
}

func getGitHubClient() (*gh.Client, error) {
	token := discoverGitHubAPIToken()
	if token == "" {
		return nil, errNoGitHubToken
	}
	var err error
	once.Do(func() {
		lazyGithubClient, err = gh.NewClient(token)
	})
	return lazyGithubClient, err
}
