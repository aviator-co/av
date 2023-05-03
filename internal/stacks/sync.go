package stacks

import (
	"fmt"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
)

type SyncStatus int

const (
	// SyncAlreadyUpToDate indicates that the sync was a no-op because the
	// target branch was already up-to-date with its parent.
	SyncAlreadyUpToDate SyncStatus = iota
	// SyncUpdated indicates that the sync updated the target branch
	// (i.e., created a merge commit or performed a rebase).
	SyncUpdated SyncStatus = iota
	// SyncConflict indicates that there was a conflict while syncing the
	// target branch with its parent.
	SyncConflict SyncStatus = iota
	// SyncNotInProgress indicates that there was no sync in progress when
	// SyncContinue was invoked.
	SyncNotInProgress SyncStatus = iota
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
	// The branch that is being synced (this should already be checked out).
	Branch string
	// The name of the parent branch
	Parent string
	// The base commit to use for the rebase (every commit *after* this one
	// will be replayed onto Parent).
	Base string
	// The strategy for performing the sync.
	// If the branch is already up-to-date, the sync will be a no-op and
	// strategy will be ignored.
	Strategy SyncStrategy
}

// SyncResult is the result of a SyncBranch operation.
type SyncResult struct {
	Status SyncStatus
	Hint   string
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
		out, err := repo.Run(&git.RunOpts{Args: []string{"merge", "-m", msg, "--log", parentHead}})
		if err != nil {
			return nil, err
		}
		if out.ExitCode != 0 {
			return &SyncResult{
				Status: SyncConflict,
				Hint:   string(out.Stderr),
			}, nil
		}
		return &SyncResult{
			Status: SyncUpdated,
		}, nil

	case StrategyRebase:
		if opts.Base == "" {
			opts.Base = mergeBase
		}
		out, err := repo.Rebase(git.RebaseOpts{
			Upstream: opts.Base,
			Onto:     opts.Parent,
			Branch:   opts.Branch,
		})
		if err != nil {
			return nil, err
		}
		if out.ExitCode != 0 {
			return &SyncResult{
				Status: SyncConflict,
				Hint:   string(out.Stderr),
			}, nil
		}
		return &SyncResult{
			Status: SyncUpdated,
		}, nil

	default:
		return nil, errors.New("unknown sync strategy")
	}
}

func SyncContinue(repo *git.Repo, strategy SyncStrategy) (*SyncResult, error) {
	switch strategy {
	case StrategyMergeCommit:
		// When merging, we just need to commit the result, assuming the
		// user hasn't created any commits in the meantime. If they *have*
		// already commited the merge, then it will be handled by the
		// already-up-to-date check above.
		out, err := repo.Run(&git.RunOpts{
			Args: []string{"commit", "--no-edit"},
		})
		if err != nil {
			return nil, err
		}
		if out.ExitCode != 0 {
			return &SyncResult{
				Status: SyncConflict,
				Hint:   string(out.Stderr),
			}, nil
		}
		return &SyncResult{
			Status: SyncUpdated,
		}, nil
	case StrategyRebase:
		out, err := repo.Rebase(git.RebaseOpts{
			Continue: true,
		})
		if err != nil {
			return nil, err
		}
		return parseRebaseOutput(out)
	default:
		return nil, errors.New("unknown sync strategy")
	}
}

func parseRebaseOutput(out *git.Output) (*SyncResult, error) {
	stderr := string(out.Stderr)

	if out.ExitCode == 0 {
		return &SyncResult{Status: SyncUpdated}, nil
	}

	// Heuristic: output when rebase is not in progress is usually
	// "    fatal: No rebase in progress?"
	if strings.Contains(stderr, "No rebase in progress") {
		return &SyncResult{
			Status: SyncNotInProgress,
			Hint:   string(out.Stderr),
		}, nil
	}

	if strings.Contains(stderr, "Could not apply") {
		return &SyncResult{
			Status: SyncConflict,
			Hint:   string(out.Stderr),
		}, nil
	}

	if out.ExitCode != 0 {
		return &SyncResult{
			Status: SyncConflict,
			Hint:   stderr,
		}, nil
	}
	return &SyncResult{
		Status: SyncUpdated,
	}, nil
}
