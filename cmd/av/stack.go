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
	deprecatedStackBranchCommitCmd := deprecateCommand(
		*stackBranchCommitCmd,
		"av commit -b",
		"branch-commit",
	)
	deprecatedStackBranchCommitCmd.Aliases = []string{"branchcommit", "bc"}
	deprecatedSubmitCmd := deprecateCommand(*stackSubmitCmd, "av pr --all", "submit")
	deprecatedSwitchCmd := deprecateCommand(*switchCmd, "av switch", "switch")
	deprecatedTidyCmd := deprecateCommand(*tidyCmd, "av tidy", "tidy")
	deprecatedTreeCmd := deprecateCommand(*treeCmd, "av tree", "tree")
	deprecatedTreeCmd.Aliases = []string{"t"}

	deprecatedStackBranchCommitCmd.Flags().
		StringVarP(&stackBranchCommitFlags.Message, "message", "m", "", "the commit message")
	deprecatedStackBranchCommitCmd.Flags().
		StringVarP(&stackBranchCommitFlags.BranchName, "branch-name", "b", "",
			"the branch name to create (if empty, automatically generated from the message)")
	deprecatedStackBranchCommitCmd.Flags().
		BoolVarP(&stackBranchCommitFlags.All, "all", "A", false, "automatically stage all files")
	deprecatedStackBranchCommitCmd.Flags().
		BoolVarP(&stackBranchCommitFlags.AllModified, "all-modified", "a", false,
			"automatically stage modified and deleted files (ignore untracked files)")

	deprecatedStackBranchCommitCmd.MarkFlagsMutuallyExclusive("all", "all-modified")

	deprecatedSubmitCmd.Flags().BoolVar(
		&stackSubmitFlags.Current, "current", false,
		"only create pull requests up to the current branch",
	)
	deprecatedSubmitCmd.Flags().BoolVar(
		&stackSubmitFlags.Draft, "draft", false,
		"create pull requests in draft mode",
	)

	stackCmd.AddCommand(
		deprecatedBranchCmd,
		deprecatedDiffCmd,
		deprecatedNextCmd,
		deprecatedPrevCmd,
		deprecatedReorderCmd,
		deprecatedReparentCmd,
		deprecatedStackBranchCommitCmd,
		deprecatedStackSyncCmd,
		deprecatedSubmitCmd,
		deprecatedSwitchCmd,
		deprecatedTidyCmd,
		deprecatedTreeCmd,
		stackAdoptCmd,
		stackForEachCmd,
		stackOrphanCmd,
		stackRestackCmd,
	)
}
