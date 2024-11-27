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
	deprecatedDiffCmd := deprecateCommand(*diffCmd, "av diff", "diff")
	deprecatedNextCmd := deprecateCommand(*nextCmd, "av next", "next")
	deprecatedNextCmd.Aliases = []string{"n"}
	deprecatedPrevCmd := deprecateCommand(*prevCmd, "av prev", "prev")
	deprecatedPrevCmd.Aliases = []string{"p"}
	deprecatedReorderCmd := deprecateCommand(*reorderCmd, "av reorder", "reorder")
	deprecatedReparentCmd := deprecateCommand(*reparentCmd, "av reparent", "reparent")
	deprecatedStackSyncCmd := deprecateCommand(*syncCmd, "av sync", "sync")
	deprecatedSwitchCmd := deprecateCommand(*switchCmd, "av switch", "switch")
	deprecatedTidyCmd := deprecateCommand(*tidyCmd, "av tidy", "tidy")
	deprecatedTreeCmd := deprecateCommand(*treeCmd, "av tree", "tree")
	deprecatedTreeCmd.Aliases = []string{"t"}

	stackCmd.AddCommand(
		deprecatedBranchCmd,
		deprecatedDiffCmd,
		deprecatedNextCmd,
		deprecatedPrevCmd,
		deprecatedReorderCmd,
		deprecatedReparentCmd,
		deprecatedStackSyncCmd,
		deprecatedSwitchCmd,
		deprecatedTidyCmd,
		deprecatedTreeCmd,
		stackAdoptCmd,
		stackBranchCommitCmd,
		stackForEachCmd,
		stackOrphanCmd,
		stackRestackCmd,
		stackSubmitCmd,
	)
}
