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

	stackCmd.AddCommand(
		deprecatedStackSyncCmd,
		stackAdoptCmd,
		stackBranchCmd,
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
