package main

import (
	"fmt"
	"strings"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
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

		rootNodes := stackutils.BuildStackTree(tx, currentBranch)
		np := &nodePrinter{repo, tx}
		for _, node := range rootNodes {
			np.printNode(0, currentBranch, true, node)
		}
		return nil
	},
}

type StackTreeBranchInfo struct {
	Deleted         bool
	NeedSync        bool
	PullRequestLink string
}

type nodePrinter struct {
	repo *git.Repo
	tx   meta.ReadTx
}

func (np *nodePrinter) printNode(columns int, currentBranchName string, isTrunk bool, node *stackutils.StackTreeNode) {
	for i, child := range node.Children {
		np.printNode(columns+i, currentBranchName, false, child)
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
	sbi := np.getBranchInfo(branch.BranchName)
	fmt.Printf(" %s", boldString(color.GreenString(branch.BranchName)))
	var stats []string
	if branch.BranchName == currentBranchName {
		stats = append(stats, boldString(color.CyanString("HEAD")))
	}
	if sbi.Deleted {
		stats = append(stats, boldString(color.RedString("deleted")))
	}
	if sbi.NeedSync {
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
		if sbi.PullRequestLink != "" {
			fmt.Print(" " + color.HiBlackString(sbi.PullRequestLink))
		} else {
			fmt.Print(" No pull request")
		}
		fmt.Println()
	}
}

func (np *nodePrinter) getBranchInfo(branchName string) *StackTreeBranchInfo {
	bi, _ := np.tx.Branch(branchName)
	branchInfo := StackTreeBranchInfo{}
	if bi.PullRequest != nil && bi.PullRequest.Permalink != "" {
		branchInfo.PullRequestLink = bi.PullRequest.Permalink
	}
	if _, err := np.repo.RevParse(&git.RevParse{Rev: branchName}); err != nil {
		branchInfo.Deleted = true
	}

	parentHead, err := np.repo.RevParse(&git.RevParse{Rev: bi.Parent.Name})
	if err != nil {
		// The parent branch doesn't exist.
		branchInfo.NeedSync = true
	} else {
		mergeBase, err := np.repo.MergeBase(&git.MergeBase{
			Revs: []string{parentHead, branchName},
		})
		if err != nil {
			// The merge base doesn't exist. This is odd. Mark the branch as needing
			// sync to see if we can fix this.
			branchInfo.NeedSync = true
		}
		if mergeBase != parentHead {
			// This branch is not on top of the parent branch. Need sync.
			branchInfo.NeedSync = true
		}
	}

	upstreamExists, err := np.repo.DoesRemoteBranchExist(branchName)
	if err != nil || !upstreamExists {
		// Not pushed.
		branchInfo.NeedSync = true
	}
	upstreamBranch := fmt.Sprintf("remotes/origin/%s", branchName)
	upstreamDiff, err := np.repo.Diff(&git.DiffOpts{
		Quiet:      true,
		Specifiers: []string{branchName, upstreamBranch},
	})
	if err != nil || !upstreamDiff.Empty {
		branchInfo.NeedSync = true
	}
	return &branchInfo
}
