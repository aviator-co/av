package main

import (
	"emperror.dev/errors"
	"github.com/spf13/cobra"
	"strconv"
)

var stackCmd = &cobra.Command{
	Use:   "stack",
	Short: "managed stacked pull requests",
}

var stackBranchFlags struct {
	// The parent branch to base the new branch off.
	// By default, this is the current branch.
	Parent string
}
var stackBranchCmd = &cobra.Command{
	Use:   "branch <branch name>",
	Short: "create a new stacked branch",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("branch name is required")
		}
		return errors.New("unimplemented")
	},
}

var stackSyncFlags struct {
	// Set the parent of the current branch to this branch.
	// This effectively re-roots the stack on a new parent.
	Parent string
	// If set, incorporate changes from the trunk (repo base branch) into the stack.
	// Only valid if synchronizing the root of a stack.
	// This effectively re-roots the stack on the latest commit from the trunk.
	Trunk bool
	// If set, do not push to GitHub.
	NoPush bool
	// If set, we're continuing a previous sync.
	// TODO:
	// 	 we might not actually need this, we can probably detect that
	//   a sync needs to be completed automagically and do the right thing.
	Continue bool
}
var stackSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "synchronize all stacked branches after the current branch",
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("unimplemented")
	},
}

var stackTreeFlags struct {
	// Print the stack starting at this branch.
	// If not set, we start at the base branch of the repository.
	Root string
	// Only recurse up-to the given depth.
	Depth int
}
var stackTreeCmd = &cobra.Command{
	Use:   "tree",
	Short: "show the tree of stacked branches",
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("unimplemented")
	},
}

var stackNextCmd = &cobra.Command{
	Use:   "next <n>",
	Short: "checkout the next branch in the stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		var n int = 1
		if len(args) == 1 {
			var err error
			n, err = strconv.Atoi(args[0])
			if err != nil {
				return errors.New("invalid number")
			}
		} else if len(args) > 1 {
			_ = cmd.Usage()
			return errors.New("too many arguments")
		}

		if n <= 0 {
			return errors.New("invalid number (must be >= 1)")
		}

		return errors.New("unimplemented")
	},
}

var stackPrevCmd = &cobra.Command{
	Use:   "prev <n>",
	Short: "checkout the previous branch in the stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		var n int = 1
		if len(args) == 1 {
			var err error
			n, err = strconv.Atoi(args[0])
			if err != nil {
				return errors.Wrap(err, "invalid number")
			}
		} else if len(args) > 1 {
			_ = cmd.Usage()
			return errors.New("too many arguments")
		}

		if n <= 0 {
			return errors.New("invalid number (must be >= 1)")
		}

		return errors.New("unimplemented")
	},
}

var stackStackStatus = &cobra.Command{
	Use:   "status",
	Short: "show the status of the stack",
	Long:  `Show the status of the stack.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: This should display information like:
		//   - a condensed description of the position within the stack
		//   - whether or not the ancestors/descendants are synchronized
		//   - how many diverging commits there are to trunk
		return errors.New("unimplemented")
	},
}

func init() {
	stackCmd.AddCommand(
		stackBranchCmd,
		stackSyncCmd,
		stackTreeCmd,
		stackNextCmd,
		stackPrevCmd,
		stackStackStatus,
	)

	// av stack branch
	stackBranchCmd.Flags().StringVar(
		&stackBranchFlags.Parent, "parent", "",
		"parent branch to stack on",
	)

	// av stack sync
	stackSyncCmd.Flags().StringVar(
		&stackSyncFlags.Parent, "parent", "",
		"set the stack parent to this branch",
	)
	stackSyncCmd.Flags().BoolVar(
		&stackSyncFlags.NoPush, "no-push", false,
		"do not force-push updated branches to GitHub",
	)
	stackSyncCmd.Flags().BoolVar(
		&stackSyncFlags.Trunk, "trunk", false,
		"synchronize the trunk into the stack",
	)
	stackSyncCmd.Flags().BoolVar(
		&stackSyncFlags.Continue, "continue", false,
		"continue a previous sync",
	)

	// av stack tree
	stackTreeCmd.Flags().StringVar(
		&stackTreeFlags.Root, "root", "",
		"only show the stack tree starting at this branch",
	)
	stackTreeCmd.Flags().IntVar(
		&stackTreeFlags.Depth, "depth", -1,
		"only show the stack tree up to this depth",
	)
}
