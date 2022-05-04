package stacks

import (
	"emperror.dev/errors"
	"fmt"
	"github.com/aviator-co/av/internal/git"
)

type SyncStatus int

const (
	// SyncAlreadyUpToDate indicates that the sync was a no-op because the
	// target branch was already up-to-date with its parent.
	SyncAlreadyUpToDate SyncStatus = iota
	// SyncUpdated indicates that the sync updated the target branch
	// (i.e., created a merge commit or perfomed a rebase).
	SyncUpdated SyncStatus = iota
	// SyncConflict indicates that there was a conflict while syncing the
	// target branch with its parent.
	SyncConflict SyncStatus = iota
)

type SyncStrategy int

const (
	// StrategyMergeCommit indicates that the sync should create a merge commit
	// from the parent branch onto the target branch.
	StrategyMergeCommit SyncStrategy = iota
	// StrategyRebase indicates that the sync should perform a rebase onto the
	// parent branch.
	StrategyRebase SyncStrategy = iota
)

type SyncBranchOpts struct {
	// The branch to sync with.
	Parent string
	// The strategy for performing the sync.
	// If the branch is already up-to-date, the sync will be a no-op and
	// strategy will be ignored.
	Strategy SyncStrategy
}

// SyncResult is the result of a SyncBranc operation.
type SyncResult struct {
	Status SyncStatus
}

// SyncBranch synchronizes the currently checked-out branch with the parent.
// The target branch is said to be already synchronized (up-to-date) if the
// target branch contains all the commits from the parent branch.
func SyncBranch(
	repo *git.Repo,
	opts *SyncBranchOpts,
) (*SyncResult, error) {
	// Determine whether or not the two branches are up-to-date.
	// If they are, we can skip the sync.
	// The target branch is up-to-date if
	//     merge-base(target, parent) = head(parent)
	// since merge-base returns the most recent common ancestor of the two.
	parentHead, err := repo.RevParse(&git.RevParse{
		Rev: opts.Parent,
	})
	if err != nil {
		return nil, errors.WrapIff(err, "failed to determine HEAD of parent branch %q", opts.Parent)
	}
	mergeBase, err := repo.MergeBase(&git.MergeBase{
		Revs: []string{parentHead, "HEAD"},
	})
	if err != nil {
		return nil, errors.WrapIff(err, "failed to determine merge base of parent branch %q and HEAD", opts.Parent)
	}
	if parentHead == mergeBase {
		return &SyncResult{
			Status: SyncAlreadyUpToDate,
		}, nil
	}

	switch opts.Strategy {
	case StrategyMergeCommit:
		msg := fmt.Sprintf("Update stacked branch to latest from %q", opts.Parent)
		_, err = repo.Git("merge", "-m", msg, "--log", parentHead)
		if err != nil {
			return &SyncResult{
				Status: SyncConflict,
			}, nil
		}

		return &SyncResult{
			Status: SyncUpdated,
		}, nil

	case StrategyRebase:
		_, err = repo.Git("rebase", "--onto", parentHead, "HEAD")
		if err != nil {
			return &SyncResult{
				Status: SyncConflict,
			}, nil
		}
		return &SyncResult{
			Status: SyncUpdated,
		}, nil

	default:
		return nil, errors.New("unknown sync strategy")
	}
}
