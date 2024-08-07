package treedetector

import (
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/go-git/go-git/v5/plumbing"
)

// GetChildren returns the children of the given branch.
func GetChildren(
	pieces map[plumbing.ReferenceName]*BranchPiece,
	branch plumbing.ReferenceName,
) map[plumbing.ReferenceName]*BranchPiece {
	ret := map[plumbing.ReferenceName]*BranchPiece{}
	var childFn func(plumbing.ReferenceName)
	childFn = func(branch plumbing.ReferenceName) {
		for bn, piece := range pieces {
			if piece.Parent == branch {
				ret[bn] = piece
				childFn(bn)
			}
		}
	}
	childFn(branch)
	return ret
}

// ConvertToStackTree converts the branch pieces to a tree structure.
func ConvertToStackTree(
	db meta.DB,
	pieces map[plumbing.ReferenceName]*BranchPiece,
	currentBranch plumbing.ReferenceName,
	sortCurrent bool,
) []*stackutils.StackTreeNode {
	trunks := map[string]bool{}
	var branches []*stackutils.StackTreeBranchInfo
	for bn, piece := range pieces {
		branches = append(branches, &stackutils.StackTreeBranchInfo{
			BranchName:       bn.Short(),
			ParentBranchName: piece.Parent.Short(),
		})
		if piece.ParentIsTrunk {
			trunks[piece.Parent.Short()] = true
		}
	}
	allBranches := db.ReadTx().AllBranches()
	for _, br := range allBranches {
		branches = append(branches, &stackutils.StackTreeBranchInfo{
			BranchName:       br.Name,
			ParentBranchName: br.Parent.Name,
		})
		if br.Parent.Trunk {
			trunks[br.Parent.Name] = true
		}
	}
	for branch := range trunks {
		branches = append(branches, &stackutils.StackTreeBranchInfo{
			BranchName:       branch,
			ParentBranchName: "",
		})
	}
	return stackutils.BuildTree(currentBranch.String(), branches, sortCurrent)
}
