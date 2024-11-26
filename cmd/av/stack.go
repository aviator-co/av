package main

import (
	"github.com/spf13/cobra"
)

var stackCmd = &cobra.Command{
	Use:     "stack",
	Aliases: []string{"st"},
	Short:   "Manage stacked pull requests",
}

func init() {
	deprecatedStackSyncCmd := deprecateCommand(*syncCmd, "av sync", "sync")

	deprecatedBranchCmd := deprecateCommand(*branchCmd, "av branch", "branch")
	deprecatedBranchCmd.Aliases = []string{"b", "br"}

	stackCmd.AddCommand(
		deprecatedBranchCmd,
		deprecatedStackSyncCmd,
		stackAdoptCmd,
		stackBranchCommitCmd,
		stackDiffCmd,
		stackForEachCmd,
		stackNextCmd,
		stackOrphanCmd,
		stackPrevCmd,
		stackReorderCmd,
		stackReparentCmd,
		stackRestackCmd,
		stackSubmitCmd,
		stackSwitchCmd,
		stackTidyCmd,
		stackTreeCmd,
	)
}
