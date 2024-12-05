package main

import (
	"fmt"
	"strings"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Show the tree of stacked branches",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		db, err := getDB(repo)
		if err != nil {
			return err
		}

		status, err := repo.Status()
		if err != nil {
			return err
		}

		var ss []string
		currentBranch := status.CurrentBranch
		tx := db.ReadTx()
		rootNodes := stackutils.BuildStackTreeAllBranches(tx, currentBranch, true)
		for _, node := range rootNodes {
			ss = append(
				ss,
				stackutils.RenderTree(node, func(branchName string, isTrunk bool) string {
					return renderStackTreeBranchInfo(
						tx,
						stackTreeStackBranchInfoStyles,
						currentBranch,
						branchName,
						isTrunk,
					)
				}),
			)
		}
		var ret string
		if len(ss) != 0 {
			ret = lipgloss.NewStyle().MarginTop(1).MarginBottom(1).Render(
				lipgloss.JoinVertical(0, ss...),
			) + "\n"
		}
		fmt.Print(ret)
		return nil
	},
}

type stackBranchInfoStyles struct {
	BranchName      lipgloss.Style
	HEAD            lipgloss.Style
	PullRequestLink lipgloss.Style
}

var stackTreeStackBranchInfoStyles = stackBranchInfoStyles{
	BranchName:      lipgloss.NewStyle().Bold(true).Foreground(colors.Green600),
	HEAD:            lipgloss.NewStyle().Bold(true).Foreground(colors.Cyan600),
	PullRequestLink: lipgloss.NewStyle(),
}

func renderStackTreeBranchInfo(
	tx meta.ReadTx,
	styles stackBranchInfoStyles,
	currentBranchName string,
	branchName string,
	isTrunk bool,
) string {
	bi, _ := tx.Branch(branchName)

	sb := strings.Builder{}
	sb.WriteString(styles.BranchName.Render(branchName))
	var stats []string
	if branchName == currentBranchName {
		stats = append(stats, styles.HEAD.Render("HEAD"))
	}
	if len(stats) > 0 {
		sb.WriteString(" (")
		sb.WriteString(strings.Join(stats, ", "))
		sb.WriteString(")")
	}

	if !isTrunk {
		sb.WriteString("\n")
		if bi.PullRequest != nil && bi.PullRequest.Permalink != "" {
			sb.WriteString(styles.PullRequestLink.Render(bi.PullRequest.Permalink))
		} else {
			sb.WriteString(styles.PullRequestLink.Render("No pull request"))
		}
	}
	return sb.String()
}

type stackTreeBranchInfo struct {
	BranchName      string
	Deleted         bool
	NeedSync        bool
	PullRequestLink string
}

func getStackTreeBranchInfo(
	repo *git.Repo,
	tx meta.ReadTx,
	branchName string,
) *stackTreeBranchInfo {
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
		mergeBase, err := repo.MergeBase(parentHead, branchName)
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
