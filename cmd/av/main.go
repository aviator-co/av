package main

import (
	"fmt"
	"github.com/aviator-co/av/internal/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
)

var rootFlags struct {
	Debug bool
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
	rootCmd.PersistentFlags().BoolVar(&rootFlags.Debug, "debug", false, "enable verbose debug logging")
	rootCmd.AddCommand(
		versionCmd,
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		format := "error: %s\n"

		// In debug mode, show more detailed information about the error
		// (including the stack trace if using pkg/errors).
		if rootFlags.Debug {
			format = "error: %#+v\n"
		}

		_, _ = fmt.Fprintf(os.Stderr, format, err)
		os.Exit(1)
	}
}
