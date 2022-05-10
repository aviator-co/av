package main

import (
	"emperror.dev/errors"
	"fmt"
	"github.com/aviator-co/av/internal/stacks"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
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
			_ = cmd.Usage()
			return errors.New("exactly one branch name is required")
		}
		repo, err := getRepo()
		if err != nil {
			return err
		}
		err = stacks.CreateBranch(repo, &stacks.CreateBranchOpts{
			Name: args[0],
		})
		if err != nil {
			return err
		}
		fmt.Printf("Created branch %s\n", args[0])
		return nil
	},
}

var stackTreeCmd = &cobra.Command{
	Use:   "tree",
	Short: "show the tree of stacked branches",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}
		trees, err := stacks.GetTrees(repo)
		if err != nil {
			return err
		}
		for _, tree := range trees {
			printStackTree(tree, 0)
		}
		return nil
	},
}

func printStackTree(tree *stacks.Tree, depth int) {
	indent := strings.Repeat("    ", depth)
	_, _ = fmt.Printf("%s%s\n", indent, tree.Branch.Name)
	for _, next := range tree.Next {
		printStackTree(next, depth+1)
	}
}

var stackNextFlags struct {
	// If set, synchronize changes from the parent branch after checking out
	// the next branch.
	Sync bool
}
var stackNextCmd = &cobra.Command{
	Use:   "next <n>",
	Short: "checkout the next branch in the stack",
	Long: strings.TrimSpace(`
Checkout the next branch in the stack.

If the --sync flag is given, this command will also synchronize changes from the
parent branch (i.e., the current branch before this command is run) into the
child branch (without recursively syncing further descendants).
`),
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

func init() {
	stackCmd.AddCommand(
		stackBranchCmd,
		stackSyncCmd,
		stackTreeCmd,
		stackNextCmd,
		stackPrevCmd,
	)

	// av stack branch
	stackBranchCmd.Flags().StringVar(
		&stackBranchFlags.Parent, "parent", "",
		"parent branch to stack on",
	)

	// av stack next
	stackNextCmd.Flags().BoolVar(
		&stackNextFlags.Sync, "sync", false,
		"synchronize changes from the parent branch",
	)
}
