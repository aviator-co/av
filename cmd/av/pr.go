package main

import (
	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var prCmd = &cobra.Command{
	Use: "pr",
}

var prCreateFlags struct {
	Base string
}
var prCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "create a pull request for the current branch",
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.GitHub.Token == "" {
			logrus.Info(
				"GitHub token is not configured. " +
					"Consider adding it to your config file (at ~/config/av/config.yaml) " +
					"to allow av to automatically create pull requests.",
			)
		}
		return errors.New("unimplemented")
	},
}

func init() {
	prCmd.AddCommand(prCreateCmd)

	// av pr create
	prCreateCmd.Flags().StringVar(
		&prCreateFlags.Base, "base", "",
		"base branch to create the pull request against",
	)
}
