package main

import (
	"github.com/spf13/cobra"
)

var stackCmd = &cobra.Command{
	Use:   "stack",
	Short: "managed stacked pull requests",
}

func init() {
	stackCmd.AddCommand(
		stackBranchCmd,
		stackBranchCommitCmd,
		stackNextCmd,
		stackPrevCmd,
		stackReparentCmd,
		stackSyncCmd,
		stackSubmitCmd,
		stackTidyCmd,
		stackTreeCmd,
	)
}
