package main

import (
	"fmt"
	"strings"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/spf13/cobra"
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

		var currentBranch string
		if dh, err := repo.DetachedHead(); err != nil {
			return err
		} else if !dh {
			currentBranch, err = repo.CurrentBranchName()
			if err != nil {
				return err
			}
		}

		// TODO[polish]:
		// 		We should show information about whether or not each branch is
		//     	up-to-date with its stack parent as well as whether or not the
		//		branch is up-to-date with its upstream tracking branch.
		if currentBranch == defaultBranch {
			_, _ = fmt.Print(
				colors.Success("* "), colors.Success(defaultBranch), "\n",
			)
		} else {
			fmt.Println(defaultBranch)
		}
		for branch, branchMeta := range branches {
			if !branchMeta.IsStackRoot() {
				continue
			}
			printStackTree(repo, branches, currentBranch, branch, 1)
		}

		return nil
	},
}

func printStackTree(repo *git.Repo, branches map[string]meta.Branch, currentBranch string, root string, depth int) {
	indent := strings.Repeat("    ", depth)
	branch, ok := branches[root]
	if !ok {
		fmt.Printf("%s<ERROR: unknown branch: %s>\n", indent, root)
		return
	}
	if currentBranch == branch.Name {
		_, _ = fmt.Print(
			indent, colors.Success("* "), colors.Success(branch.Name), "\n",
		)
	} else {
		_, _ = fmt.Printf("%s%s\n", indent, branch.Name)
	}
	for _, next := range branch.Children {
		printStackTree(repo, branches, currentBranch, next, depth+1)
	}
}
