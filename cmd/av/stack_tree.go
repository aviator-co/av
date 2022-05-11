package main

import (
	"fmt"
	"github.com/aviator-co/av/internal/stacks"
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
