package planner

import (
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/sequencer"
	"github.com/go-git/go-git/v5/plumbing"
)

func PlanForRestack(tx meta.ReadTx, repo *git.Repo, targetBranches []plumbing.ReferenceName) ([]sequencer.RestackOp, error) {
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
