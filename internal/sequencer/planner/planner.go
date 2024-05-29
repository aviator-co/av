package planner

import (
	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/sequencer"
	"github.com/go-git/go-git/v5/plumbing"
)

func PlanForRestack(tx meta.ReadTx, repo *git.Repo, currentBranch plumbing.ReferenceName, restackAll, restackCurrent bool) ([]sequencer.RestackOp, error) {
	var targetBranches []plumbing.ReferenceName
	var err error
	if restackAll {
		targetBranches, err = GetTargetBranches(tx, repo, false, AllBranches)
	} else if restackCurrent {
		targetBranches, err = GetTargetBranches(tx, repo, false, CurrentAndParents)
	} else {
		targetBranches, err = GetTargetBranches(tx, repo, false, CurrentStack)
	}
	if err != nil {
		return nil, err
	}

	var ret []sequencer.RestackOp
	for _, br := range targetBranches {
		avbr, _ := tx.Branch(br.Short())
		if avbr.MergeCommit != "" {
			// Skip rebasing branches that have merge commits.
			continue
		}
		ret = append(ret, sequencer.RestackOp{
			Name:             br,
			NewParent:        plumbing.NewBranchReferenceName(avbr.Parent.Name),
			NewParentIsTrunk: avbr.Parent.Trunk,
		})
	}
	return ret, nil
}

func PlanForSync(tx meta.ReadTx, repo *git.Repo, targetBranches []plumbing.ReferenceName, syncToTrunkInsteadOfMergeCommit bool) ([]sequencer.RestackOp, error) {
	var ret []sequencer.RestackOp
	for _, br := range targetBranches {
		avbr, _ := tx.Branch(br.Short())
		if avbr.MergeCommit != "" {
			// Skip rebasing branches that have merge commits.
			continue
		}
		if !avbr.Parent.Trunk {
			// Check if the parent branch is merged.
			avpbr, _ := tx.Branch(avbr.Parent.Name)
			if avpbr.MergeCommit != "" {
				// The parent is merged. Sync to either trunk or merge commit.
				trunk, _ := meta.Trunk(tx, br.Short())
				var newParentHash plumbing.Hash
				if syncToTrunkInsteadOfMergeCommit {
					// By setting this to ZeroHash, the sequencer will sync to
					// the remote tracking branch.
					newParentHash = plumbing.ZeroHash
				} else {
					newParentHash = plumbing.NewHash(avpbr.MergeCommit)
				}
				ret = append(ret, sequencer.RestackOp{
					Name:             br,
					NewParent:        plumbing.NewBranchReferenceName(trunk),
					NewParentIsTrunk: true,
					NewParentHash:    newParentHash,
				})
				continue
			}
		}
		ret = append(ret, sequencer.RestackOp{
			Name:             br,
			NewParent:        plumbing.NewBranchReferenceName(avbr.Parent.Name),
			NewParentIsTrunk: avbr.Parent.Trunk,
		})
	}
	return ret, nil
}

func PlanForReparent(tx meta.ReadTx, repo *git.Repo, currentBranch, newParentBranch plumbing.ReferenceName) ([]sequencer.RestackOp, error) {
	if newParentBranch == currentBranch {
		return nil, errors.New("cannot re-parent to self")
	}
	children := meta.SubsequentBranches(tx, currentBranch.Short())
	for _, child := range children {
		if child == newParentBranch.Short() {
			return nil, errors.New("cannot re-parent to a child branch")
		}
	}
	isParentTrunk, err := repo.IsTrunkBranch(newParentBranch.Short())
	if err != nil {
		return nil, err
	}
	var ret []sequencer.RestackOp
	ret = append(ret, sequencer.RestackOp{
		Name:             currentBranch,
		NewParent:        newParentBranch,
		NewParentIsTrunk: isParentTrunk,
	})
	for _, child := range children {
		avbr, _ := tx.Branch(child)
		if avbr.MergeCommit != "" {
			// Skip rebasing branches that have merge commits.
			continue
		}
		ret = append(ret, sequencer.RestackOp{
			Name:             plumbing.NewBranchReferenceName(child),
			NewParent:        plumbing.NewBranchReferenceName(avbr.Parent.Name),
			NewParentIsTrunk: avbr.Parent.Trunk,
		})
	}
	return ret, nil
}
