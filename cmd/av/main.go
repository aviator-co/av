package main

import (
	"emperror.dev/errors"
	"fmt"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/git"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"strings"
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

		repo, _ := getRepo()
		var configDirs []string
		if repo != nil {
			gitDir, err := repo.Git("rev-parse", "--git-dir")
			if err != nil {
				logrus.WithError(err).Warning("failed to determine git root directory")
			} else {
				configDirs = append(configDirs, gitDir)
			}
		}
		if _, err := config.Load(configDirs); err != nil {
			return errors.Wrap(err, "failed to load configuration")
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
		authCmd,
		prCmd,
		stackCmd,
		versionCmd,
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {

		// In debug mode, show more detailed information about the error
		// (including the stack trace if using pkg/errors).
		if rootFlags.Debug {
			stackTrace := fmt.Sprintf("%+v", err)
			_, _ = fmt.Fprintf(os.Stderr, "error: %s\n%s\n", err, indent(stackTrace, "\t"))
		} else {
			_, _ = fmt.Fprintf(os.Stderr, "error: %s\n", err)
		}

		os.Exit(1)
	}
}

func indent(s string, prefix string) string {
	// why is this not in the stdlib????
	return prefix + strings.Replace(s, "\n", "\n"+prefix, -1)
}

var cachedRepo *git.Repo

func getRepo() (*git.Repo, error) {
	if cachedRepo == nil {
		cmd := exec.Command("git", "rev-parse", "--show-toplevel")
		if rootFlags.Directory != "" {
			cmd.Dir = rootFlags.Directory
		}
		toplevel, err := cmd.Output()
		if err != nil {
			return nil, errors.Wrap(err, "failed to determine repo toplevel")
		}
		cachedRepo, err = git.OpenRepo(strings.TrimSpace(string(toplevel)))
		if err != nil {
			return nil, errors.Wrap(err, "failed to open git repo")
		}
	}
	return cachedRepo, nil
}
