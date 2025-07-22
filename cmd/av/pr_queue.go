package main

import (
	"github.com/spf13/cobra"
)

var prQueueCmd = &cobra.Command{
	Use:          "queue",
	Short:        "Queue an existing pull request for the current branch",
	Hidden:       true,
	SilenceUsage: true,
	Args:         cobra.NoArgs,
	// error or reterr from emperror.dev/errors here?
	RunE: func(cmd *cobra.Command, _ []string) error {
		return queue(cmd.Context())
	},
}
