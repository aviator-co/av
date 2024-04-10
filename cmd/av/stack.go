package main

import (
	"github.com/spf13/cobra"
)

var stackCmd = &cobra.Command{
	Use:     "stack",
	Aliases: []string{"st"},
	Short:   "managed stacked pull requests",
}

func init() {
	stackCmd.AddCommand(
		stackBranchCmd,
		stackBranchCommitCmd,
		stackDiffCmd,
		stackForEachCmd,
		stackNextCmd,
		stackPrevCmd,
		stackOrphanCmd,
		stackReorderCmd,
		stackReparentCmd,
		stackSyncCmd,
		stackSubmitCmd,
		stackTidyCmd,
		stackTreeCmd,
	)
}
