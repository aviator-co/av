package stackutils

import (
	"fmt"
	"sort"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
)

type StackTreeBranchInfo struct {
	BranchName        string
	Deleted           bool
	NeedSync          bool
	PullRequestNumber int64
	PullRequestLink   string

	parentBranchName string
}

type StackTreeNode struct {
	Branch   *StackTreeBranchInfo
	Children []*StackTreeNode
}

func buildTree(currentBranchName string, branches []*StackTreeBranchInfo, sortCurrent bool) []*StackTreeNode {
	childBranches := make(map[string][]string)
	branchMap := make(map[string]*StackTreeNode)
	for _, branch := range branches {
		branchMap[branch.BranchName] = &StackTreeNode{Branch: branch}
		childBranches[branch.parentBranchName] = append(childBranches[branch.parentBranchName], branch.BranchName)
	}
	for _, branch := range branches {
		node := branchMap[branch.BranchName]
		for _, childBranch := range childBranches[branch.BranchName] {
			node.Children = append(node.Children, branchMap[childBranch])
		}
	}

	// Find the root branches.
	var rootBranches []*StackTreeNode
	for _, branch := range branches {
		if branch.parentBranchName == "" || branchMap[branch.parentBranchName] == nil {
			rootBranches = append(rootBranches, branchMap[branch.BranchName])
		}
	}

	// Find the path that contains the current branch.
	currentBranchPath := make(map[string]bool)
	var currentBranchVisitFn func(node *StackTreeNode) bool
	currentBranchVisitFn = func(node *StackTreeNode) bool {
		if node.Branch.BranchName == currentBranchName {
			currentBranchPath[node.Branch.BranchName] = true
			return true
		}
		for _, child := range node.Children {
			if currentBranchVisitFn(child) {
				currentBranchPath[node.Branch.BranchName] = true
				return true
			}
		}
		return false
	}
	if sortCurrent {
		for _, rootBranch := range rootBranches {
			currentBranchVisitFn(rootBranch)
		}
	}
	for _, node := range branchMap {
		// Visit the current branch first. Otherwise, use alphabetical order for the initial ones.
		sort.Slice(node.Children, func(i, j int) bool {
			if currentBranchPath[node.Children[i].Branch.BranchName] {
				return true
			}
			if currentBranchPath[node.Children[j].Branch.BranchName] {
				return false
			}
			return node.Children[i].Branch.BranchName < node.Children[j].Branch.BranchName
		})
	}
	sort.Slice(rootBranches, func(i, j int) bool {
		if currentBranchPath[rootBranches[i].Branch.BranchName] {
			return true
		}
		if currentBranchPath[rootBranches[j].Branch.BranchName] {
			return false
		}
		return rootBranches[i].Branch.BranchName < rootBranches[j].Branch.BranchName
	})
	return rootBranches
}

func BuildStackTree(repo *git.Repo, tx meta.ReadTx, currentBranch string) []*StackTreeNode {
	return buildStackTree(repo, currentBranch, tx.AllBranches(), true)
}

func BuildStackTreeForPullRequest(repo *git.Repo, tx meta.ReadTx, currentBranch string) (*StackTreeNode, error) {
	branchesToInclude, err := meta.StackBranchesMap(tx, currentBranch)
	if err != nil {
		return nil, err
	}

	// Don't sort based on the current branch so that the output is consistent between branches.
	stackTree := buildStackTree(repo, currentBranch, branchesToInclude, false)
	if len(stackTree) != 1 {
		return nil, fmt.Errorf("expected one root branch, got %d", len(stackTree))
	}

	return stackTree[0], nil
}

func buildStackTree(repo *git.Repo, currentBranch string, branchesToInclude map[string]meta.Branch, sortCurrent bool) []*StackTreeNode {
	trunks := map[string]bool{}
	var branches []*StackTreeBranchInfo
	for _, branch := range branchesToInclude {
		branches = append(branches, getBranchInfo(repo, branch))
		if branch.Parent.Trunk {
			trunks[branch.Parent.Name] = true
		}
	}
	for branch := range trunks {
		branches = append(branches, &StackTreeBranchInfo{
			BranchName:       branch,
			parentBranchName: "",
			NeedSync:         false,
			Deleted:          false,
		})
	}
	return buildTree(currentBranch, branches, sortCurrent)
}

func getBranchInfo(repo *git.Repo, branch meta.Branch) *StackTreeBranchInfo {
	branchInfo := StackTreeBranchInfo{
		BranchName:       branch.Name,
		parentBranchName: branch.Parent.Name,
	}
	if branch.PullRequest != nil && branch.PullRequest.Number != 0 {
		branchInfo.PullRequestNumber = branch.PullRequest.Number
	}
	if branch.PullRequest != nil && branch.PullRequest.Permalink != "" {
		branchInfo.PullRequestLink = branch.PullRequest.Permalink
	}
	if _, err := repo.RevParse(&git.RevParse{Rev: branch.Name}); err != nil {
		branchInfo.Deleted = true
	}

	parentHead, err := repo.RevParse(&git.RevParse{Rev: branch.Parent.Name})
	if err != nil {
		// The parent branch doesn't exist.
		branchInfo.NeedSync = true
	} else {
		mergeBase, err := repo.MergeBase(&git.MergeBase{
			Revs: []string{parentHead, branch.Name},
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

	upstreamExists, err := repo.DoesRemoteBranchExist(branch.Name)
	if err != nil || !upstreamExists {
		// Not pushed.
		branchInfo.NeedSync = true
	}
	upstreamBranch := fmt.Sprintf("remotes/origin/%s", branch.Name)
	upstreamDiff, err := repo.Diff(&git.DiffOpts{
		Quiet:      true,
		Specifiers: []string{branch.Name, upstreamBranch},
	})
	if err != nil || !upstreamDiff.Empty {
		branchInfo.NeedSync = true
	}
	return &branchInfo
}
