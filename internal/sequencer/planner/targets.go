package planner

import (
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/go-git/go-git/v5/plumbing"
)

type TargetBranchMode int

const (
	// Target all branches in the repository.
	AllBranches TargetBranchMode = iota
	// The current branch and all its predecessors.
	CurrentAndParents
	// The current branch and all its successors.
	CurrentAndChildren
	// Branches of the current stack. (The stack root and all its successors.)
	CurrentStack
)

// GetTargetBranches returns the branches to be restacked.
//
// If `includeStackRoots` is true, the stack root branches (the immediate children of the trunk
// branches) are included in the result.
func GetTargetBranches(tx meta.ReadTx, repo *git.Repo, includeStackRoots bool, mode TargetBranchMode) ([]plumbing.ReferenceName, error) {
	var ret []plumbing.ReferenceName
	if mode == AllBranches {
		for _, br := range tx.AllBranches() {
			if !br.IsStackRoot() {
				continue
			}
			if includeStackRoots {
				ret = append(ret, plumbing.NewBranchReferenceName(br.Name))
			}
			for _, n := range meta.SubsequentBranches(tx, br.Name) {
				ret = append(ret, plumbing.NewBranchReferenceName(n))
			}
		}
		return ret, nil
	}
	if mode == CurrentAndParents {
		curr, err := repo.CurrentBranchName()
		if err != nil {
			return nil, err
		}
		prevs, err := meta.PreviousBranches(tx, curr)
		if err != nil {
			return nil, err
		}
		for _, n := range prevs {
			br, _ := tx.Branch(n)
			if !br.IsStackRoot() || includeStackRoots {
				ret = append(ret, plumbing.NewBranchReferenceName(n))
			}
		}
		br, _ := tx.Branch(curr)
		if !br.IsStackRoot() || includeStackRoots {
			ret = append(ret, plumbing.NewBranchReferenceName(curr))
		}
		return ret, nil
	}
	if mode == CurrentAndChildren {
		curr, err := repo.CurrentBranchName()
		if err != nil {
			return nil, err
		}
		br, _ := tx.Branch(curr)
		if !br.IsStackRoot() || includeStackRoots {
			ret = append(ret, plumbing.NewBranchReferenceName(curr))
		}
		// The rest of the branches cannot be a stack root.
		for _, n := range meta.SubsequentBranches(tx, curr) {
			ret = append(ret, plumbing.NewBranchReferenceName(n))
		}
		return ret, nil
	}
	curr, err := repo.CurrentBranchName()
	if err != nil {
		return nil, err
	}
	brs, err := meta.StackBranches(tx, curr)
	if err != nil {
		return nil, err
	}
	for _, n := range brs {
		br, _ := tx.Branch(n)
		if !br.IsStackRoot() || includeStackRoots {
			ret = append(ret, plumbing.NewBranchReferenceName(n))
		}
	}
	return ret, nil
}
