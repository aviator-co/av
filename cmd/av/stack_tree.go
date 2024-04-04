package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
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

		trunks := map[string]bool{}
		var branches []*stackTreeBranchInfo
		for _, branch := range tx.AllBranches() {
			branches = append(branches, getBranchInfo(repo, branch))
			if branch.Parent.Trunk {
				trunks[branch.Parent.Name] = true
			}
		}
		for branch := range trunks {
			branches = append(branches, &stackTreeBranchInfo{
				branchName:       branch,
				parentBranchName: "",
				needSync:         false,
				deleted:          false,
			})
		}
		rootNodes := buildTree(currentBranch, branches)
		for _, node := range rootNodes {
			printNode(0, currentBranch, true, node)
		}
		return nil
	},
}

type stackTreeBranchInfo struct {
	branchName       string
	parentBranchName string
	pullRequestLink  string
	needSync         bool
	deleted          bool
}

func getBranchInfo(repo *git.Repo, branch meta.Branch) *stackTreeBranchInfo {
	branchInfo := stackTreeBranchInfo{
		branchName:       branch.Name,
		parentBranchName: branch.Parent.Name,
	}
	if branch.PullRequest != nil && branch.PullRequest.Permalink != "" {
		branchInfo.pullRequestLink = branch.PullRequest.Permalink
	}
	if _, err := repo.RevParse(&git.RevParse{Rev: branch.Name}); err != nil {
		branchInfo.deleted = true
	}

	parentHead, err := repo.RevParse(&git.RevParse{Rev: branch.Parent.Name})
	if err != nil {
		// The parent branch doesn't exist.
		branchInfo.needSync = true
	} else {
		mergeBase, err := repo.MergeBase(&git.MergeBase{
			Revs: []string{parentHead, branch.Name},
		})
		if err != nil {
			// The merge base doesn't exist. This is odd. Mark the branch as needing
			// sync to see if we can fix this.
			branchInfo.needSync = true
		}
		if mergeBase != parentHead {
			// This branch is not on top of the parent branch. Need sync.
			branchInfo.needSync = true
		}
	}

	upstreamExists, err := repo.DoesRemoteBranchExist(branch.Name)
	if err != nil || !upstreamExists {
		// Not pushed.
		branchInfo.needSync = true
	}
	upstreamBranch := fmt.Sprintf("remotes/origin/%s", branch.Name)
	upstreamDiff, err := repo.Diff(&git.DiffOpts{
		Quiet:      true,
		Specifiers: []string{branch.Name, upstreamBranch},
	})
	if err != nil || !upstreamDiff.Empty {
		branchInfo.needSync = true
	}
	return &branchInfo
}

type stackTreeNode struct {
	branch   *stackTreeBranchInfo
	children []*stackTreeNode
}

func buildTree(currentBranchName string, branches []*stackTreeBranchInfo) []*stackTreeNode {
	childBranches := make(map[string][]string)
	branchMap := make(map[string]*stackTreeNode)
	for _, branch := range branches {
		branchMap[branch.branchName] = &stackTreeNode{branch: branch}
		childBranches[branch.parentBranchName] = append(childBranches[branch.parentBranchName], branch.branchName)
	}
	for _, branch := range branches {
		node := branchMap[branch.branchName]
		for _, childBranch := range childBranches[branch.branchName] {
			node.children = append(node.children, branchMap[childBranch])
		}
	}

	// Find the root branches.
	var rootBranches []*stackTreeNode
	for _, branch := range branches {
		if branch.parentBranchName == "" || branchMap[branch.parentBranchName] == nil {
			rootBranches = append(rootBranches, branchMap[branch.branchName])
		}
	}

	// Find the path that contains the current branch.
	currentBranchPath := make(map[string]bool)
	var currentBranchVisitFn func(node *stackTreeNode) bool
	currentBranchVisitFn = func(node *stackTreeNode) bool {
		if node.branch.branchName == currentBranchName {
			currentBranchPath[node.branch.branchName] = true
			return true
		}
		for _, child := range node.children {
			if currentBranchVisitFn(child) {
				currentBranchPath[node.branch.branchName] = true
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
		sort.Slice(node.children, func(i, j int) bool {
			if currentBranchPath[node.children[i].branch.branchName] {
				return true
			}
			if currentBranchPath[node.children[j].branch.branchName] {
				return false
			}
			return node.children[i].branch.branchName < node.children[j].branch.branchName
		})
	}
	sort.Slice(rootBranches, func(i, j int) bool {
		if currentBranchPath[rootBranches[i].branch.branchName] {
			return true
		}
		if currentBranchPath[rootBranches[j].branch.branchName] {
			return false
		}
		return rootBranches[i].branch.branchName < rootBranches[j].branch.branchName
	})
	return rootBranches
}

func printNode(columns int, currentBranchName string, isTrunk bool, node *stackTreeNode) {
	for i, child := range node.children {
		printNode(columns+i, currentBranchName, false, child)
	}
	if len(node.children) > 1 {
		fmt.Print(" ")
		for i := 0; i < columns; i++ {
			fmt.Print(" │")
		}
		fmt.Print(" ├")
		for i := 0; i < len(node.children)-2; i++ {
			fmt.Print("─┴")
		}
		fmt.Print("─┘")
		fmt.Println()
	} else if len(node.children) == 1 {
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
	branch := node.branch
	fmt.Printf(" %s", boldString(color.GreenString(branch.branchName)))
	var stats []string
	if branch.branchName == currentBranchName {
		stats = append(stats, boldString(color.CyanString("HEAD")))
	}
	if branch.deleted {
		stats = append(stats, boldString(color.RedString("deleted")))
	}
	if branch.needSync {
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
		if branch.pullRequestLink != "" {
			fmt.Print(" " + color.HiBlackString(branch.pullRequestLink))
		} else {
			fmt.Print(" No pull request")
		}
		fmt.Println()
	}
}
