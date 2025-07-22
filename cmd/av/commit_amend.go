package main

import (
	"github.com/spf13/cobra"
)

var commitAmendFlags struct {
	// The commit message to update with.
	Message string
	NoEdit  bool
	All     bool
}

var commitAmendCmd = &cobra.Command{
	Use:   "amend",
	Short: "Amend a commit",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return amendCmd(
			cmd.Context(),
			commitAmendFlags.Message,
			!commitAmendFlags.NoEdit,
			commitAmendFlags.All,
		)
	},
}
