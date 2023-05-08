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

		db, err := getDB(repo)
		if err != nil {
			return err
		}
		tx := db.ReadTx()

		defaultBranch, err := repo.DefaultBranch()
		if err != nil {
			return err
		}

		branches := tx.AllBranches()

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

	branchInfo, err := getBranchInfo(repo, branch)
	if err != nil {
		fmt.Printf("<ERROR: cannot get branch info: %v>\n", err)
		return
	}

	if currentBranch == branch.Name {
		_, _ = fmt.Print(
			indent, colors.Success("* "), colors.Success(branch.Name), " ", colors.Faint(branchInfo), "\n",
		)
	} else {
		_, _ = fmt.Print(indent, branch.Name, " ", colors.Faint(branchInfo), "\n")
	}
	for _, next := range branch.Children {
		printStackTree(repo, branches, currentBranch, next, depth+1)
	}
}

func getBranchInfo(repo *git.Repo, branch meta.Branch) (string, error) {
	var branchInfo string

	parentStatus, err := getParentStatus(repo, branch)
	if err != nil {
		return "", err
	}

	upstreamStatus, err := getUpstreamStatus(repo, branch)
	if err != nil {
		return "", err
	}

	branchStatus := strings.Trim(fmt.Sprintf("%s, %s", parentStatus, upstreamStatus), ", ")
	if branchStatus != "" {
		branchInfo = fmt.Sprintf("(%s)", branchStatus)
	}

	if branch.PullRequest != nil && branch.PullRequest.Permalink != "" {
		branchInfo = branch.PullRequest.Permalink + " " + branchInfo
	}

	return branchInfo, nil
}

// Check if branch is up to date with the parent branch.
func getParentStatus(repo *git.Repo, branch meta.Branch) (string, error) {
	parentHead, err := repo.RevParse(&git.RevParse{Rev: branch.Parent.Name})
	if err != nil {
		return "", err
	}

	mergeBase, err := repo.MergeBase(&git.MergeBase{
		Revs: []string{parentHead, branch.Name},
	})
	if err != nil {
		return "", err
	}
	if mergeBase == parentHead {
		return "", nil
	}
	
	return "needs sync", nil
}

// Check if branch is up to date with the upstream branch.
// This is doing `git diff <givenBranch> remotes/origin/<givenBranch>`
func getUpstreamStatus(repo *git.Repo, branch meta.Branch) (string, error) {
	upstreamExists, err := repo.DoesRemoteBranchExist(branch.Name)
	if err != nil || !upstreamExists {
		return "not pushed", nil
	}

	upstreamBranch := fmt.Sprintf("remotes/origin/%s", branch.Name)
	upstreamDiff, err := repo.Diff(&git.DiffOpts{Quiet: true, Branch1: branch.Name, Branch2: upstreamBranch})
	if err != nil {
		return "", err
	} 
	
	if upstreamDiff.Empty {
		return "", nil
	}
		
	return "not pushed", nil
}
