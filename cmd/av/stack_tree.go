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
		for _, node := range rootNodes {
			fmt.Print(stackutils.RenderTree(node, func(branchName string, isTrunk bool) string {
				stbi := getStackTreeBranchInfo(repo, tx, branchName)
				return renderStackTreeBranchInfo(stbi, currentBranch, branchName, isTrunk)
			}))
		}
		return nil
	},
}

func renderStackTreeBranchInfo(stbi *stackTreeBranchInfo, currentBranchName string, branchName string, isTrunk bool) string {
	sb := strings.Builder{}
	sb.WriteString(boldString(color.GreenString(branchName)))
	var stats []string
	if branchName == currentBranchName {
		stats = append(stats, boldString(color.CyanString("HEAD")))
	}
	if stbi.Deleted {
		stats = append(stats, boldString(color.RedString("deleted")))
	}
	if !isTrunk && stbi.NeedSync {
		stats = append(stats, boldString(color.RedString("need sync")))
	}
	if len(stats) > 0 {
		sb.WriteString(" (")
		sb.WriteString(strings.Join(stats, ", "))
		sb.WriteString(")")
	}
	sb.WriteString("\n")

	if !isTrunk {
		if stbi.PullRequestLink != "" {
			sb.WriteString(color.HiBlackString(stbi.PullRequestLink))
		} else {
			sb.WriteString("No pull request")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

type stackTreeBranchInfo struct {
	BranchName      string
	Deleted         bool
	NeedSync        bool
	PullRequestLink string
}

func getStackTreeBranchInfo(repo *git.Repo, tx meta.ReadTx, branchName string) *stackTreeBranchInfo {
	bi, _ := tx.Branch(branchName)
	branchInfo := stackTreeBranchInfo{
		BranchName: branchName,
	}
	if bi.PullRequest != nil && bi.PullRequest.Permalink != "" {
		branchInfo.PullRequestLink = bi.PullRequest.Permalink
	}
	if _, err := repo.RevParse(&git.RevParse{Rev: branchName}); err != nil {
		branchInfo.Deleted = true
	}

	parentHead, err := repo.RevParse(&git.RevParse{Rev: bi.Parent.Name})
	if err != nil {
		// The parent branch doesn't exist.
		branchInfo.NeedSync = true
	} else {
		mergeBase, err := repo.MergeBase(&git.MergeBase{
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

	upstreamExists, err := repo.DoesRemoteBranchExist(branchName)
	if err != nil || !upstreamExists {
		// Not pushed.
		branchInfo.NeedSync = true
	}
	upstreamBranch := fmt.Sprintf("remotes/origin/%s", branchName)
	upstreamDiff, err := repo.Diff(&git.DiffOpts{
		Quiet:      true,
		Specifiers: []string{branchName, upstreamBranch},
	})
	if err != nil || !upstreamDiff.Empty {
		branchInfo.NeedSync = true
	}
	return &branchInfo
}
