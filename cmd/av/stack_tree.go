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

		rootNodes := stackutils.BuildStackTreeAllBranches(tx, currentBranch, true)
		for _, node := range rootNodes {
			fmt.Println(stackutils.RenderTree(node, func(branchName string, isTrunk bool) string {
				stbi := getStackTreeBranchInfo(repo, tx, branchName)
				return renderStackTreeBranchInfo(stackTreeStackBranchInfoStyles, stbi, currentBranch, branchName, isTrunk)
			}))
		}
		return nil
	},
}

type stackBranchInfoStyles struct {
	BranchName      lipgloss.Style
	HEAD            lipgloss.Style
	Deleted         lipgloss.Style
	NeedSync        lipgloss.Style
	PullRequestLink lipgloss.Style
}

var stackTreeStackBranchInfoStyles = stackBranchInfoStyles{
	BranchName:      lipgloss.NewStyle().Bold(true).Foreground(colors.Green600),
	HEAD:            lipgloss.NewStyle().Bold(true).Foreground(colors.Cyan600),
	Deleted:         lipgloss.NewStyle().Bold(true).Foreground(colors.Red700),
	NeedSync:        lipgloss.NewStyle().Bold(true).Foreground(colors.Red700),
	PullRequestLink: lipgloss.NewStyle(),
}

func renderStackTreeBranchInfo(styles stackBranchInfoStyles, stbi *stackTreeBranchInfo, currentBranchName string, branchName string, isTrunk bool) string {
	sb := strings.Builder{}
	sb.WriteString(styles.BranchName.Render(branchName))
	var stats []string
	if branchName == currentBranchName {
		stats = append(stats, styles.HEAD.Render("HEAD"))
	}
	if stbi.Deleted {
		stats = append(stats, styles.Deleted.Render("deleted"))
	}
	if !isTrunk && stbi.NeedSync {
		stats = append(stats, styles.NeedSync.Render("need sync"))
	}
	if len(stats) > 0 {
		sb.WriteString(" (")
		sb.WriteString(strings.Join(stats, ", "))
		sb.WriteString(")")
	}
	sb.WriteString("\n")

	if !isTrunk {
		if stbi.PullRequestLink != "" {
			sb.WriteString(styles.PullRequestLink.Render(stbi.PullRequestLink))
		} else {
			sb.WriteString(styles.PullRequestLink.Render("No pull request"))
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
