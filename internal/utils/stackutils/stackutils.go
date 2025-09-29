package stackutils

import (
	"fmt"
	"slices"
	"sort"

	"github.com/aviator-co/av/internal/meta"
)

type StackTreeBranchInfo struct {
	BranchName       string
	ParentBranchName string
}

type StackTreeNode struct {
	Branch   *StackTreeBranchInfo
	Children []*StackTreeNode
}

func BuildTree(
	currentBranchName string,
	branches []*StackTreeBranchInfo,
	sortCurrent bool,
) []*StackTreeNode {
	childBranches := make(map[string][]string)
	branchMap := make(map[string]*StackTreeNode)
	for _, branch := range branches {
		branchMap[branch.BranchName] = &StackTreeNode{Branch: branch}
		childBranches[branch.ParentBranchName] = append(
			childBranches[branch.ParentBranchName],
			branch.BranchName,
		)
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
		if branch.ParentBranchName == "" || branchMap[branch.ParentBranchName] == nil {
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
		if slices.ContainsFunc(node.Children, currentBranchVisitFn) {
			currentBranchPath[node.Branch.BranchName] = true
			return true
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

func BuildStackTreeAllBranches(
	tx meta.ReadTx,
	currentBranch string,
	sortCurrent bool,
) []*StackTreeNode {
	return buildStackTree(currentBranch, tx.AllBranches(), sortCurrent)
}

func BuildStackTreeCurrentStack(
	tx meta.ReadTx,
	currentBranch string,
	sortCurrent bool,
) (*StackTreeNode, error) {
	nodes, err := BuildStackTreeRelatedBranchStacks(
		tx,
		currentBranch,
		sortCurrent,
		[]string{currentBranch},
	)
	if err != nil {
		return nil, err
	}
	if len(nodes) != 1 {
		return nil, fmt.Errorf("expected one root branch, got %d", len(nodes))
	}
	return nodes[0], nil
}

func BuildStackTreeRelatedBranchStacks(
	tx meta.ReadTx,
	currentBranch string,
	sortCurrent bool,
	relatedBranches []string,
) ([]*StackTreeNode, error) {
	branches := map[string]bool{}
	for _, branch := range relatedBranches {
		stack, err := meta.StackBranches(tx, branch)
		if err != nil {
			// Ignore branches that are not adopted to av.
			continue
		}
		for _, name := range stack {
			branches[name] = true
		}
	}
	var names []string
	for name := range branches {
		names = append(names, name)
	}

	branchesToInclude, err := meta.BranchesMap(tx, names)
	if err != nil {
		return nil, err
	}
	return buildStackTree(currentBranch, branchesToInclude, sortCurrent), nil
}

func buildStackTree(
	currentBranch string,
	branchesToInclude map[string]meta.Branch,
	sortCurrent bool,
) []*StackTreeNode {
	trunks := map[string]bool{}
	var branches []*StackTreeBranchInfo
	for _, branch := range branchesToInclude {
		branches = append(branches, &StackTreeBranchInfo{
			BranchName:       branch.Name,
			ParentBranchName: branch.Parent.Name,
		})
		if branch.Parent.Trunk {
			trunks[branch.Parent.Name] = true
		}
	}
	for branch := range trunks {
		branches = append(branches, &StackTreeBranchInfo{
			BranchName:       branch,
			ParentBranchName: "",
		})
	}
	return BuildTree(currentBranch, branches, sortCurrent)
}
