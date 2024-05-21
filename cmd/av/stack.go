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
		stackOrphanCmd,
		stackPrevCmd,
		stackReorderCmd,
		stackReparentCmd,
		stackSubmitCmd,
		stackSwitchCmd,
		stackSyncCmd,
		stackTidyCmd,
		stackTreeCmd,
	)
}
