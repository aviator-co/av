package actions

import (
	"context"
	"emperror.dev/errors"
	"fmt"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/sirupsen/logrus"
	"os"
)

type SyncBranchOpts struct {
	Branch  string
	NoFetch bool
	NoPush  bool
	// If specified, synchronize the branch against the latest version of the
	// trunk branch. This value is ignored if the branch is not a stack root.
	ToTrunk bool

	Continuation *SyncBranchContinuation
}

type SyncBranchResult struct {
	git.RebaseResult

	// If set, the sync needs to be continued.
	// This is set if and only if RebaseResult.Status is RebaseConflict
	Continuation *SyncBranchContinuation

	// The updated branch metadata (if the rebase was successful)
	Branch meta.Branch
}

type (
	SyncBranchContinuation struct {
		// The original HEAD commit of the branch.
		OldHead string `json:"oldHead"`
		// The commit that we were rebasing the branch on top of.
		ParentCommit string `json:"parentCommit"`

		// If set, we need to re-assign the branch to be a stack root that is
		// based on this trunk branch.
		NewTrunk string `json:"newTrunk,omitempty"`
	}
)

// SyncBranch synchronizes a branch with its parent.
func SyncBranch(
	ctx context.Context, repo *git.Repo, client *gh.Client,
	repoMeta meta.Repository, opts SyncBranchOpts,
) (*SyncBranchResult, error) {
	branch, _ := meta.ReadBranch(repo, opts.Branch)
	_, _ = fmt.Fprint(os.Stderr, "Synchronizing branch ", colors.UserInput(branch.Name), "...\n")

	if opts.Continuation != nil {
		return syncBranchContinue(ctx, repo, opts, branch)
	}

	if !opts.NoFetch {
		update, err := UpdatePullRequestState(ctx, repo, client, repoMeta, branch.Name)
		if err != nil {
			_, _ = fmt.Fprint(os.Stderr, colors.Failure("      - error: ", err.Error()), "\n")
			return nil, errors.Wrap(err, "failed to fetch latest PR info")
		}
		if update.Changed {
			_, _ = fmt.Fprint(os.Stderr, "      - found updated pull request: ", colors.UserInput(update.Pull.Permalink), "\n")
		}
		branch = update.Branch
		if branch.PullRequest == nil {
			_, _ = fmt.Fprint(os.Stderr,
				"      - this branch does not have an open pull request"+
					" (create one with ", colors.CliCmd("av pr create"),
				" or ", colors.CliCmd("av stack submit"), ")\n",
			)
		}
	}

	if branch.MergeCommit != "" {
		_, _ = fmt.Fprint(os.Stderr,
			"  - skipping sync for merged branch "+
				"(merged in commit ", colors.UserInput(git.ShortSha(branch.MergeCommit)), ")"+
				"\n",
		)
		return &SyncBranchResult{
			RebaseResult: git.RebaseResult{Status: git.RebaseAlreadyUpToDate},
		}, nil
	}

	branchHead, err := repo.RevParse(&git.RevParse{Rev: branch.Name})
	if err != nil {
		return nil, errors.WrapIff(err, "failed to get head of branch %q", branch.Name)
	}

	if branch.IsStackRoot() {
		trunk := branch.Parent.Name
		if !opts.ToTrunk {
			_, _ = fmt.Fprint(os.Stderr,
				"  - branch is a stack root, nothing to do",
				" (run ", colors.CliCmd("av stack sync --trunk"),
				" to sync against the latest commit in ", colors.UserInput(trunk), ")\n",
			)
			return &SyncBranchResult{
				RebaseResult: git.RebaseResult{Status: git.RebaseAlreadyUpToDate},
			}, nil
		}

		// First, try to fetch latest commit from the trunk...
		_, _ = fmt.Fprint(os.Stderr,
			"  - fetching latest commit from ", colors.UserInput("origin/", trunk), "\n",
		)
		if _, err := repo.Run(&git.RunOpts{
			Args: []string{"fetch", "origin", trunk},
		}); err != nil {
			_, _ = fmt.Fprint(os.Stderr,
				"  - ",
				colors.Failure("error: failed to fetch HEAD of "), colors.UserInput(trunk),
				colors.Failure(" from origin: ", err.Error()), "\n",
			)
			return nil, errors.WrapIff(err, "failed to fetch trunk branch %q from origin", trunk)
		}

		trunkHead, err := repo.RevParse(&git.RevParse{Rev: trunk})
		if err != nil {
			return nil, errors.WrapIff(err, "failed to get HEAD of %q", trunk)
		}

		rebase, err := repo.RebaseParse(git.RebaseOpts{
			Branch:   opts.Branch,
			Upstream: trunkHead,
		})
		if err != nil {
			return nil, err
		}
		msgRebaseResult(rebase)
		//nolint:exhaustive
		switch rebase.Status {
		case git.RebaseConflict:
			return &SyncBranchResult{
				*rebase,
				&SyncBranchContinuation{
					OldHead: branchHead,
				},
				branch,
			}, nil
		default:
			return &SyncBranchResult{*rebase, nil, branch}, nil
		}
	}

	// We have three possibilities here:
	//   1. The parent branch has been merged. We need to rebase this branch
	//      on top of the commit that was actually merged into the trunk.
	//   2. The branch is up-to-date with its parent. This is defined as
	//      merge-base(branch, parent) = head(parent).
	//   3. The branch is not up-to-date with its parent (usually this means
	//      that a commit was added to parent in the meantime, but can also
	//      happen if the parent branch was rebased itself).

	// Scenario 1: the parent branch has been merged.
	parent, _ := meta.ReadBranch(repo, branch.Parent.Name)
	if parent.MergeCommit != "" {
		short := git.ShortSha(parent.MergeCommit)
		_, _ = fmt.Fprint(os.Stderr,
			"  - parent ", colors.UserInput(branch.Parent.Name),
			" (pull ", colors.UserInput("#", parent.PullRequest.GetNumber()), ")",
			" was merged\n",
		)
		_, _ = fmt.Fprint(os.Stderr,
			"  - rebasing ", colors.UserInput(branch.Name),
			" on top of merge commit ", colors.UserInput(short), "\n",
		)
		if _, err := repo.Git("fetch", "origin", branch.MergeCommit); err != nil {
			return nil, errors.WrapIff(err, "failed to fetch merge commit %q from origin", short)
		}

		rebase, err := repo.RebaseParse(git.RebaseOpts{
			Branch:   branch.Name,
			Upstream: branch.Parent.Name,
			// Replay the commits from this branch directly onto the merge commit.
			// The HEAD of trunk might have moved forward since this, but this is
			// probably the best thing to do here (we bias towards introducing as
			// few unrelated commits into the history as possible -- we have to
			// introduce everything that landed in trunk before the merge commit,
			// but we hold off on introducing anything that landed after).
			// The user can always run `av stack sync --trunk` to sync against the
			// tip of master.
			// For example if we have
			//        A---B---M---C---D  main
			//         \     /
			//          Q---R  stacked-1
			//               \
			//                X---Y  stacked-2
			// (where M is the commit that merged stacked-1 into main, **even
			// if it's actually a squash merge and not a real merge commit),
			// then after the sync we'll have
			//        A---B---M---C---D  main
			//                 \
			//                  X'--Y'  stacked-2
			// Note that we've introduced B into the history of stacked-2, but
			// not C or D since those commits come after M.
			Onto: parent.MergeCommit,
		})
		if err != nil {
			return nil, err
		}
		if rebase.Status == git.RebaseConflict {
			return &SyncBranchResult{
				*rebase,
				&SyncBranchContinuation{
					OldHead:      branchHead,
					ParentCommit: parent.MergeCommit,
					NewTrunk:     parent.Parent.Name,
				},
				branch,
			}, nil
		}

		return &SyncBranchResult{*rebase, nil, branch}, nil
	}

	// Scenario 2: the branch is up-to-date with its parent.
	parentHead, err := repo.RevParse(&git.RevParse{Rev: parent.Name})
	if err != nil {
		return nil, errors.WrapIff(err, "failed to resolve HEAD of parent branch %q", parent.Name)
	}
	mergeBase, err := repo.MergeBase(&git.MergeBase{
		Revs: []string{parentHead, branch.Name},
	})
	if err != nil {
		return nil, errors.WrapIff(err, "failed to compute merge base of %q and %q", parent.Name, branch.Name)
	}
	if mergeBase == parentHead {
		_, _ = fmt.Fprint(os.Stderr,
			"  - already up-to-date with parent ", colors.UserInput(parent.Name),
			"\n",
		)
		return &SyncBranchResult{git.RebaseResult{Status: git.RebaseAlreadyUpToDate}, nil, branch}, nil
	}

	// Scenario 3: the branch is not up-to-date with its parent.
	_, _ = fmt.Fprint(os.Stderr,
		"  - synching branch ", colors.UserInput(branch.Name),
		" on latest commit ", git.ShortSha(parentHead),
		" of parent branch ", colors.UserInput(parent.Name),
		"\n",
	)
	// We need to use `rebase --onto` here and be very careful about how we
	// determine the commits that are being rebased on top of parentHead.
	// Suppose we have a history like
	// 	   A---B---C---D  main
	//      \
	//       Q---R  stacked-1
	//        \
	//         T  stacked-2
	//          \
	//           W  stacked-3
	// where R is a commit that was added to stacked-1 after stacked-2 was
	// created. After syncing stacked-2 against stacked-1, we have
	// 	   A---B---C---D  main
	//      \
	//       Q---R  stacked-1
	//        \    \
	//         \    T'  stacked-2
	//          T
	//           \
	//            W  stacked-3
	// Notice that the commit T is "orphaned" in the history. If we did a
	// `git rebase stacked-2 stacked-3` at this point, Git would determine
	// that we should play T---W on top of T'. This is why we keep track
	// of the old parent head here, so that we can tell git to replay
	// everything after T.
	// With `git rebase --onto stacked-2 T stacked-3`, Git looks at the
	// difference between T and stacked-3, determines that it's only the
	// commit W, and then plays the commit W **onto** stacked-2 (aka T').
	rebase, err := repo.RebaseParse(git.RebaseOpts{
		Branch:   branch.Name,
		Onto:     parentHead,
		Upstream: branch.Parent.Head,
	})
	if err != nil {
		return nil, err
	}
	msgRebaseResult(rebase)

	//nolint:exhaustive
	switch rebase.Status {
	case git.RebaseConflict:
		return &SyncBranchResult{
			*rebase,
			&SyncBranchContinuation{
				OldHead:      branchHead,
				ParentCommit: parentHead,
			},
			branch,
		}, nil
	case git.RebaseUpdated:
		return &SyncBranchResult{*rebase, nil, branch}, nil
	default:
		// We shouldn't even get an already-up-to-date or not-in-progress
		// here...
		logrus.Warn("unexpected rebase status: ", rebase.Status)
		return &SyncBranchResult{*rebase, nil, branch}, nil
	}
}

func syncBranchContinue(
	ctx context.Context, repo *git.Repo,
	opts SyncBranchOpts, branch meta.Branch,
) (*SyncBranchResult, error) {
	rebase, err := repo.RebaseParse(git.RebaseOpts{
		Continue: true,
	})
	if err != nil {
		return nil, err
	}

	//nolint:exhaustive
	switch rebase.Status {
	case git.RebaseNotInProgress:
		// TODO:
		//    I think we could try to detect whether or not the rebase was
		//    actually completed or just aborted, but it's whatever for right
		//    now.
		_, _ = fmt.Fprint(os.Stderr,
			"  - ", colors.Warning("WARNING: expected a rebase to be in progress"),
			" (assuming the rebase was completed with git rebase --continue)\n",
			"      - use ", colors.CliCmd("av stack sync --continue"),
			" instead of git rebase --continue to avoid this warning\n",
		)
	case git.RebaseConflict:
		msgRebaseResult(rebase)
		return &SyncBranchResult{*rebase, opts.Continuation, branch}, nil
	default:
		msgRebaseResult(rebase)
	}

	// Finish setting the new trunk for the branch
	if opts.Continuation.NewTrunk != "" {
		var err error
		branch.Parent, err = meta.ReadBranchState(repo, opts.Continuation.NewTrunk, true)
		if err != nil {
			return nil, err
		}
		_, _ = fmt.Fprint(os.Stderr,
			"  - this branch is now a stack root based on trunk branch ",
			colors.UserInput(branch.Parent.Name), "\n",
		)
		if err := meta.WriteBranch(repo, branch); err != nil {
			return nil, err
		}
	}

	return &SyncBranchResult{*rebase, nil, branch}, nil
}
