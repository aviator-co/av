package main

import (
	"github.com/spf13/cobra"
)

var stackBranchCommitFlags struct {
	// The commit message.
	Message string

	// Name of the new branch.
	BranchName string

	// Same as `git add --all`.
	// Stages all changes, including untracked files.
	All bool

	// Same as `git commit --all`.
	// Stage all files that have been modified and deleted, but ignore untracked files.
	AllModified bool
}

var stackBranchCommitCmd = &cobra.Command{
	Use:          "branch-commit",
	Short:        "Create a new branch in the stack with the staged changes",
	SilenceUsage: true,
	Args:         cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) (reterr error) {
		return branchAndCommit(
			cmd.Context(),
			stackBranchCommitFlags.BranchName,
			stackBranchCommitFlags.Message,
			stackBranchCommitFlags.All,
			stackBranchCommitFlags.AllModified,
			"",
		)
	},
}
