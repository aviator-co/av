package stackutils

import (
	"fmt"
	"sort"

	"github.com/aviator-co/av/internal/meta"
)

type StackTreeBranchInfo struct {
	BranchName       string
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

func BuildStackTree(tx meta.ReadTx, currentBranch string) []*StackTreeNode {
	return buildStackTree(currentBranch, tx.AllBranches(), true)
}

func BuildStackTreeForPullRequest(tx meta.ReadTx, currentBranch string) (*StackTreeNode, error) {
	branchesToInclude, err := meta.StackBranchesMap(tx, currentBranch)
	if err != nil {
		return nil, err
	}

	// Don't sort based on the current branch so that the output is consistent between branches.
	stackTree := buildStackTree(currentBranch, branchesToInclude, false)
	if len(stackTree) != 1 {
		return nil, fmt.Errorf("expected one root branch, got %d", len(stackTree))
	}

	return stackTree[0], nil
}

func buildStackTree(currentBranch string, branchesToInclude map[string]meta.Branch, sortCurrent bool) []*StackTreeNode {
	trunks := map[string]bool{}
	var branches []*StackTreeBranchInfo
	for _, branch := range branchesToInclude {
		branches = append(branches, &StackTreeBranchInfo{
			BranchName:       branch.Name,
			parentBranchName: branch.Parent.Name,
		})
		if branch.Parent.Trunk {
			trunks[branch.Parent.Name] = true
		}
	}
	for branch := range trunks {
		branches = append(branches, &StackTreeBranchInfo{
			BranchName:       branch,
			parentBranchName: "",
		})
	}
	return buildTree(currentBranch, branches, sortCurrent)
}
