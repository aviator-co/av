package main

import (
	"fmt"
	"github.com/aviator-co/av/internal/meta"
	"github.com/spf13/cobra"
	"strings"
)

var stackTreeCmd = &cobra.Command{
	Use:   "tree",
	Short: "show the tree of stacked branches",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		defaultBranch, err := repo.DefaultBranch()
		if err != nil {
			return err
		}

		branches, err := meta.ReadAllBranches(repo)
		if err != nil {
			return err
		}

		// TODO[polish]:
		// 		We should show information about whether or not each branch is
		//     	up-to-date with its stack parent as well as whether or not the
		//		branch is up-to-date with its upstream tracking branch.
		fmt.Println(defaultBranch)
		for branch, branchMeta := range branches {
			if branchMeta.Parent != "" {
				// not a stack root
				continue
			}
			printStackTree(branches, branch, 1)
		}

		return nil
	},
}

func printStackTree(branches map[string]meta.Branch, root string, depth int) {
	indent := strings.Repeat("    ", depth)
	branch, ok := branches[root]
	if !ok {
		fmt.Printf("%s<ERROR: unknown branch: %s>\n", indent, root)
		return
	}
	_, _ = fmt.Printf("%s%s\n", indent, branch.Name)
	for _, next := range branch.Children {
		printStackTree(branches, next, depth+1)
	}
}
