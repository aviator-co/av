package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var stackBranchCmd = &cobra.Command{
	Use:     "branch",
	Aliases: []string{"b", "br"},
	Short:   "Deprecated: Create or rename a branch in the stack (use 'av branch' instead)",
	Long: strings.TrimSpace(`
'av stack branch' is deprecated. Please use 'av branch' instead.

Create a new branch that is stacked on the current branch.

<parent-branch>. If omitted, the new branch bases off the current branch.

If the --rename/-m flag is given, the current branch is renamed to the name
given as the first argument to the command. Branches should only be renamed
with this command (not with git branch -m ...) because av needs to update
internal tracking metadata that defines the order of branches within a stack.`),
	Args: cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("'av stack branch' is deprecated. Please use 'av branch' instead.")
		return branchCmd.RunE(cmd, args)
	},
}

func init() {
	stackBranchCmd.Flags().
		StringVar(&stackBranchFlags.Parent, "parent", "", "the parent branch to base the new branch off of")
	// NOTE: We use -m as the shorthand here to match `git branch -m ...`.
	// See the comment on stackBranchFlags.Rename.
	stackBranchCmd.Flags().
		BoolVarP(&stackBranchFlags.Rename, "rename", "m", false, "rename the current branch")
	stackBranchCmd.Flags().
		BoolVar(&stackBranchFlags.Force, "force", false, "force rename the current branch, even if a pull request exists")

	_ = stackBranchCmd.RegisterFlagCompletionFunc(
		"parent",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			branches, _ := allBranches()
			return branches, cobra.ShellCompDirectiveDefault
		},
	)
}
