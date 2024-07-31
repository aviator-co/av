package treedetector

import (
	"github.com/aviator-co/av/internal/utils/sliceutils"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/go-git/go-git/v5/plumbing"
)

// GetStackRoot returns the root of the stack of branches from the given branch. It returns an empty
// string if the stack root cannot be detected.
func GetStackRoot(
	pieces map[plumbing.ReferenceName]*BranchPiece,
	branch plumbing.ReferenceName,
) plumbing.ReferenceName {
	stackRoot := branch
	for {
		piece, ok := pieces[stackRoot]
		if !ok || piece.Parent == "" {
			// Cannot detect the stack root.
			return ""
		}
		if piece.ParentIsTrunk {
			break
		}
		stackRoot = piece.Parent
	}
	return stackRoot
}

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

// GetPossibleChildren returns the possible children of the given branch.
func GetPossibleChildren(
	pieces map[plumbing.ReferenceName]*BranchPiece,
	branch plumbing.ReferenceName,
) []*BranchPiece {
	var ret []*BranchPiece
	var allChildFn func(plumbing.ReferenceName)
	var mpChildFn func(plumbing.ReferenceName)
	allChildFn = func(branch plumbing.ReferenceName) {
		for bn, piece := range pieces {
			if piece.Parent == branch || sliceutils.Contains(piece.PossibleParents, branch) {
				ret = append(ret, piece)
				allChildFn(bn)
			}
		}
	}
	mpChildFn = func(branch plumbing.ReferenceName) {
		for bn, piece := range pieces {
			if sliceutils.Contains(piece.PossibleParents, branch) {
				ret = append(ret, piece)
				allChildFn(bn)
			}
		}
	}
	mpChildFn(branch)
	return ret
}

// ConvertToStackTree converts the branch pieces to a tree structure.
func ConvertToStackTree(
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
	for branch := range trunks {
		branches = append(branches, &stackutils.StackTreeBranchInfo{
			BranchName:       branch,
			ParentBranchName: "",
		})
	}
	return stackutils.BuildTree(currentBranch.String(), branches, sortCurrent)
}
