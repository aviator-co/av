package stacks

import (
	"emperror.dev/errors"
	"fmt"
	"github.com/aviator-co/av/internal/git"
)

type SyncStatus int

const (
	SyncAlreadyUpToDate SyncStatus = iota
	SyncUpdated         SyncStatus = iota
	SyncConflict        SyncStatus = iota
)

type SyncStrategy int

const (
	StrategyMergeCommit SyncStrategy = iota
	StrategyRebase      SyncStrategy = iota
)

type SyncBranchOpts struct {
	Parent   string
	Strategy SyncStrategy
}

type SyncResult struct {
	Status SyncStatus
}

func SyncBranch(
	repo *git.Repo,
	opts *SyncBranchOpts,
) (*SyncResult, error) {
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
