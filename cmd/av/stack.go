package main

import (
	"github.com/spf13/cobra"
)

var stackCmd = &cobra.Command{
	Use:     "stack",
	Aliases: []string{"st"},
	Hidden:  true,
	Short:   "Deprecated command to manage stacked pull requests",
}

func init() {
	deprecatedAdoptCmd := deprecateCommand(*adoptCmd, "av adopt", "adopt")

	deprecatedBranchCmd := deprecateCommand(*branchCmd, "av branch", "branch")
	deprecatedBranchCmd.Aliases = []string{"b", "br"}

	deprecatedDiffCmd := deprecateCommand(*diffCmd, "av diff", "diff")

	deprecatedNextCmd := deprecateCommand(*nextCmd, "av next", "next")
	deprecatedNextCmd.Aliases = []string{"n"}

	deprecatedOrphanCmd := deprecateCommand(*orphanCmd, "av orphan", "orphan")

	deprecatedPrevCmd := deprecateCommand(*prevCmd, "av prev", "prev")
	deprecatedPrevCmd.Aliases = []string{"p"}

	deprecatedReorderCmd := deprecateCommand(*reorderCmd, "av reorder", "reorder")

	deprecatedReparentCmd := deprecateCommand(*reparentCmd, "av reparent", "reparent")

	deprecatedRestackCmd := deprecateCommand(*restackCmd, "av restack", "restack")

	deprecatedStackBranchCommitCmd := deprecateCommand(
		*stackBranchCommitCmd,
		"av commit -b",
		"branch-commit",
	)
	deprecatedStackBranchCommitCmd.Aliases = []string{"branchcommit", "bc"}
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

	deprecatedSyncCmd := deprecateCommand(*syncCmd, "av sync", "sync")
	deprecatedSyncCmd.Flags().BoolVar(
		&syncFlags.All, "all", false,
		"synchronize all branches",
	)
	deprecatedSyncCmd.Flags().BoolVar(
		&syncFlags.Current, "current", false,
		"only sync changes to the current branch\n(don't recurse into descendant branches)",
	)
	deprecatedSyncCmd.Flags().StringVar(
		&syncFlags.Push, "push", "ask",
		"push the rebased branches to the remote repository\n(ask|yes|no)",
	)
	deprecatedSyncCmd.Flags().StringVar(
		&syncFlags.Prune, "prune", "ask",
		"delete branches that have been merged into the parent branch\n(ask|yes|no)",
	)
	deprecatedSyncCmd.Flags().Lookup("prune").NoOptDefVal = "ask"
	deprecatedSyncCmd.Flags().BoolVar(
		&syncFlags.RebaseToTrunk, "rebase-to-trunk", false,
		"rebase the branches to the latest trunk always",
	)

	deprecatedSyncCmd.Flags().BoolVar(
		&syncFlags.Continue, "continue", false,
		"continue an in-progress sync",
	)
	deprecatedSyncCmd.Flags().BoolVar(
		&syncFlags.Abort, "abort", false,
		"abort an in-progress sync",
	)
	deprecatedSyncCmd.Flags().BoolVar(
		&syncFlags.Skip, "skip", false,
		"skip the current commit and continue an in-progress sync",
	)
	deprecatedSyncCmd.MarkFlagsMutuallyExclusive("current", "all")
	deprecatedSyncCmd.MarkFlagsMutuallyExclusive("continue", "abort", "skip")

	deprecatedSubmitCmd := deprecateCommand(*stackSubmitCmd, "av pr --all", "submit")
	deprecatedSubmitCmd.Flags().BoolVar(
		&stackSubmitFlags.Current, "current", false,
		"only create pull requests up to the current branch",
	)
	deprecatedSubmitCmd.Flags().BoolVar(
		&stackSubmitFlags.Draft, "draft", false,
		"create pull requests in draft mode",
	)

	deprecatedSwitchCmd := deprecateCommand(*switchCmd, "av switch", "switch")

	deprecatedTidyCmd := deprecateCommand(*tidyCmd, "av tidy", "tidy")

	deprecatedTreeCmd := deprecateCommand(*treeCmd, "av tree", "tree")
	deprecatedTreeCmd.Aliases = []string{"t"}

	stackCmd.AddCommand(
		deprecatedAdoptCmd,
		deprecatedBranchCmd,
		deprecatedDiffCmd,
		deprecatedNextCmd,
		deprecatedOrphanCmd,
		deprecatedPrevCmd,
		deprecatedReorderCmd,
		deprecatedReparentCmd,
		deprecatedStackBranchCommitCmd,
		deprecatedSubmitCmd,
		deprecatedSwitchCmd,
		deprecatedSyncCmd,
		deprecatedTidyCmd,
		deprecatedTreeCmd,
		stackForEachCmd,
		deprecatedRestackCmd,
	)
}
