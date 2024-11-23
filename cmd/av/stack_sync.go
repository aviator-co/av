package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var stackSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Deprecated: Synchronize stacked branches with GitHub (use 'av sync' instead)",
	Long: strings.TrimSpace(`
'av stack sync' is deprecated. Please use 'av sync' instead.

Synchronize stacked branches to be up-to-date with their parent branches.

By default, this command will sync all branches starting at the root of the
stack and recursively rebasing each branch based on the latest commit from the
parent branch.

If the --all flag is given, this command will sync all branches in the repository.

If the --current flag is given, this command will not recursively sync dependent
branches of the current branch within the stack. This allows you to make changes
to the current branch before syncing the rest of the stack.

If the --rebase-to-trunk flag is given, this command will synchronize changes from the
latest commit to the repository base branch (e.g., main or master) into the
stack. This is useful for rebasing a whole stack on the latest changes from the
base branch.
`),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("'av stack sync' is deprecated. Please use 'av sync' instead.")
		return syncCmd.RunE(cmd, args)
	},
}

func init() {
	stackSyncCmd.Flags().BoolVar(
		&stackSyncFlags.All, "all", false,
		"synchronize all branches",
	)
	stackSyncCmd.Flags().BoolVar(
		&stackSyncFlags.Current, "current", false,
		"only sync changes to the current branch\n(don't recurse into descendant branches)",
	)
	stackSyncCmd.Flags().StringVar(
		&stackSyncFlags.Push, "push", "ask",
		"push the rebased branches to the remote repository\n(ask|yes|no)",
	)
	stackSyncCmd.Flags().StringVar(
		&stackSyncFlags.Prune, "prune", "ask",
		"delete branches that have been merged into the parent branch\n(ask|yes|no)",
	)
	stackSyncCmd.Flags().Lookup("prune").NoOptDefVal = "ask"
	stackSyncCmd.Flags().BoolVar(
		&stackSyncFlags.RebaseToTrunk, "rebase-to-trunk", false,
		"rebase the branches to the latest trunk always",
	)

	stackSyncCmd.Flags().BoolVar(
		&stackSyncFlags.Continue, "continue", false,
		"continue an in-progress sync",
	)
	stackSyncCmd.Flags().BoolVar(
		&stackSyncFlags.Abort, "abort", false,
		"abort an in-progress sync",
	)
	stackSyncCmd.Flags().BoolVar(
		&stackSyncFlags.Skip, "skip", false,
		"skip the current commit and continue an in-progress sync",
	)
	stackSyncCmd.MarkFlagsMutuallyExclusive("current", "all")
	stackSyncCmd.MarkFlagsMutuallyExclusive("continue", "abort", "skip")

	// Deprecated flags
	stackSyncCmd.Flags().Bool("no-fetch", false,
		"(deprecated; use av stack restack for offline restacking) do not fetch the latest status from GitHub",
	)
	_ = stackSyncCmd.Flags().
		MarkDeprecated("no-fetch", "please use av stack restack for offline restacking")
	stackSyncCmd.Flags().Bool("trunk", false,
		"(deprecated; use --rebase-to-trunk to rebase all branches to trunk) rebase the stack on the trunk branch",
	)
	_ = stackSyncCmd.Flags().
		MarkDeprecated("trunk", "please use --rebase-to-trunk to rebase all branches to trunk")
	stackSyncCmd.Flags().String("parent", "",
		"(deprecated; use av stack adopt or av stack reparent) parent branch to rebase onto",
	)
	_ = stackSyncCmd.Flags().
		MarkDeprecated("parent", "please use av stack adopt or av stack reparent")
}
