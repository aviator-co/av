package main

import (
	"fmt"
	"strings"

	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var boldString = color.New(color.Bold).SprintFunc()

var stackTreeCmd = &cobra.Command{
	Use:     "tree",
	Aliases: []string{"t"},
	Short:   "show the tree of stacked branches",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		db, err := getDB(repo)
		if err != nil {
			return err
		}
		tx := db.ReadTx()

		var currentBranch string
		if dh, err := repo.DetachedHead(); err != nil {
			return err
		} else if !dh {
			currentBranch, err = repo.CurrentBranchName()
			if err != nil {
				return err
			}
		}

		rootNodes := stackutils.BuildStackTree(repo, tx, currentBranch)
		for _, node := range rootNodes {
			printNode(0, currentBranch, true, node)
		}
		return nil
	},
}

func printNode(columns int, currentBranchName string, isTrunk bool, node *stackutils.StackTreeNode) {
	for i, child := range node.Children {
		printNode(columns+i, currentBranchName, false, child)
	}
	if len(node.Children) > 1 {
		fmt.Print(" ")
		for i := 0; i < columns; i++ {
			fmt.Print(" │")
		}
		fmt.Print(" ├")
		for i := 0; i < len(node.Children)-2; i++ {
			fmt.Print("─┴")
		}
		fmt.Print("─┘")
		fmt.Println()
	} else if len(node.Children) == 1 {
		fmt.Print(" ")
		for i := 0; i < columns+1; i++ {
			fmt.Print(" │")
		}
		fmt.Println()
	} else if columns > 0 {
		fmt.Print(" ")
		for i := 0; i < columns; i++ {
			fmt.Print(" │")
		}
		fmt.Println()
	}

	fmt.Print(" ")
	for i := 0; i < columns; i++ {
		fmt.Print(" │")
	}
	fmt.Print(" *")
	branch := node.Branch
	fmt.Printf(" %s", boldString(color.GreenString(branch.BranchName)))
	var stats []string
	if branch.BranchName == currentBranchName {
		stats = append(stats, boldString(color.CyanString("HEAD")))
	}
	if branch.Deleted {
		stats = append(stats, boldString(color.RedString("deleted")))
	}
	if branch.NeedSync {
		stats = append(stats, boldString(color.RedString("need sync")))
	}
	if len(stats) > 0 {
		fmt.Print(" (")
		fmt.Print(strings.Join(stats, ", "))
		fmt.Print(")")
	}
	fmt.Println()

	if !isTrunk {
		fmt.Print(" ")
		for i := 0; i < columns+1; i++ {
			fmt.Print(" │")
		}
		if branch.PullRequestLink != "" {
			fmt.Print(" " + color.HiBlackString(branch.PullRequestLink))
		} else {
			fmt.Print(" No pull request")
		}
		fmt.Println()
	}
}
