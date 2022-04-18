package main

import (
	"github.com/aviator-co/av/internal/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
)

var rootFlags struct {
	Debug bool
}

var rootCmd = &cobra.Command{
	Use:           "av",
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if rootFlags.Debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		logrus.Debugf("av version: %s", config.Version)
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
		os.Exit(1)
	}
}
