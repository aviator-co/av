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
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(
		&rootFlags.Debug, "debug", false,
		"enable verbose debug logging",
	)
	rootCmd.PersistentFlags().StringVarP(
		&rootFlags.Directory, "", "C", "",
		"directory to use for git repository",
	)
	rootCmd.AddCommand(
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

func getRepo() (*git.Repo, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	if rootFlags.Directory != "" {
		cmd.Dir = rootFlags.Directory
	}
	toplevel, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine repo toplevel")
	}
	return git.OpenRepo(strings.TrimSpace(string(toplevel)))
}
