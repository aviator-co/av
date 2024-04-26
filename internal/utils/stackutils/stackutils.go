package stackutils

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/fatih/color"
)

type StackTreeBranchInfo struct {
	BranchName       string
	ParentBranchName string
	PullRequestLink  string
	NeedSync         bool
	Deleted          bool
}

type StackTreeNode struct {
	Branch   *StackTreeBranchInfo
	Children []*StackTreeNode
}

func buildTree(currentBranchName string, branches []*StackTreeBranchInfo) []*StackTreeNode {
	childBranches := make(map[string][]string)
	branchMap := make(map[string]*StackTreeNode)
	for _, branch := range branches {
		branchMap[branch.BranchName] = &StackTreeNode{Branch: branch}
		childBranches[branch.ParentBranchName] = append(childBranches[branch.ParentBranchName], branch.BranchName)
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
		for _, child := range node.Children {
			if currentBranchVisitFn(child) {
				currentBranchPath[node.Branch.BranchName] = true
				return true
			}
		}
		return false
	}
	for _, rootBranch := range rootBranches {
		currentBranchVisitFn(rootBranch)
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

func getBranchInfo(repo *git.Repo, branch meta.Branch) *StackTreeBranchInfo {
	branchInfo := StackTreeBranchInfo{
		BranchName:       branch.Name,
		ParentBranchName: branch.Parent.Name,
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

func BuildStackTree(repo *git.Repo, tx meta.ReadTx, currentBranch string) []*StackTreeNode {
	trunks := map[string]bool{}
	var branches []*StackTreeBranchInfo
	for _, branch := range tx.AllBranches() {
		branches = append(branches, getBranchInfo(repo, branch))
		if branch.Parent.Trunk {
			trunks[branch.Parent.Name] = true
		}
	}
	for branch := range trunks {
		branches = append(branches, &StackTreeBranchInfo{
			BranchName:       branch,
			ParentBranchName: "",
			NeedSync:         false,
			Deleted:          false,
		})
	}
	return buildTree(currentBranch, branches)
}

var boldString = color.New(color.Bold).SprintFunc()

func PrintNode(columns int, currentBranchName string, isTrunk bool, node *StackTreeNode) {
	for i, child := range node.Children {
		PrintNode(columns+i, currentBranchName, false, child)
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
