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
	deprecatedBranchCmd := deprecateCommand(*branchCmd, "av branch", "branch")
	deprecatedBranchCmd.Aliases = []string{"b", "br"}

	deprecatedNextCmd := deprecateCommand(*nextCmd, "av next", "next")
	deprecatedNextCmd.Aliases = []string{"n"}

	deprecatedPrevCmd := deprecateCommand(*prevCmd, "av prev", "prev")
	deprecatedPrevCmd.Aliases = []string{"p"}

	deprecatedStackSyncCmd := deprecateCommand(*syncCmd, "av sync", "sync")

	stackCmd.AddCommand(
		deprecatedBranchCmd,
		deprecatedNextCmd,
		deprecatedPrevCmd,
		deprecatedStackSyncCmd,
		stackAdoptCmd,
		stackBranchCommitCmd,
		stackDiffCmd,
		stackForEachCmd,
		stackOrphanCmd,
		stackReorderCmd,
		stackReparentCmd,
		stackRestackCmd,
		stackSubmitCmd,
		stackSwitchCmd,
		stackTidyCmd,
		stackTreeCmd,
	)
}
