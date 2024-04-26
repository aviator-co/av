package actions

import (
	"context"
	"fmt"
	"os"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/ghutils"
	"github.com/shurcooL/githubv4"
)

type SyncBranchOpts struct {
	Branch string
	Fetch  bool
	Push   bool
	// If specified, synchronize the branch against the latest version of the
	// trunk branch. This value is ignored if the branch is not a stack root.
	ToTrunk bool
	// If true, skip the current commit.
	// This must only be set after a rebase conflict in a sync.
	Skip bool

	Continuation *SyncBranchContinuation
}

type (
	SyncBranchContinuation struct {
		// The new parent name.
		NewParentName string `json:"parentName"`
		// If set, set this as the parent HEAD. If unset, the parent is treated as trunk.
		NewParentCommit string `json:"parentCommit"`
	}
)

// SyncBranch synchronizes a branch with its parent.
func SyncBranch(
	ctx context.Context,
	repo *git.Repo,
	client *gh.Client,
	tx meta.WriteTx,
	opts SyncBranchOpts,
) (*SyncBranchContinuation, error) {
	branch, _ := tx.Branch(opts.Branch)
	_, _ = fmt.Fprint(os.Stderr, "Synchronizing branch ", colors.UserInput(branch.Name), "...\n")

	var cont *SyncBranchContinuation
	var pull *gh.PullRequest

	if opts.Continuation != nil {
		var err error
		cont, err = syncBranchContinue(ctx, repo, tx, opts, branch)
		if err != nil {
			return nil, err
		}
	} else {
		if opts.Fetch {
			fetchHead, err := fetchRemoteTrunkHead(repo, tx, branch)
			if err != nil {
				return nil, err
			}
			update, err := UpdatePullRequestState(ctx, client, tx, branch.Name)
			if err != nil {
				_, _ = fmt.Fprint(os.Stderr, colors.Failure("      - error: ", err.Error()), "\n")
				return nil, errors.Wrap(err, "failed to fetch latest PR info")
			}
			pull = update.Pull
			if update.Changed {
				_, _ = fmt.Fprint(os.Stderr, "      - found updated pull request: ", colors.UserInput(update.Pull.Permalink), "\n")
			}
			branch, _ = tx.Branch(opts.Branch)
			if branch.PullRequest == nil {
				_, _ = fmt.Fprint(os.Stderr,
					"      - this branch does not have an open pull request"+
						" (create one with ", colors.CliCmd("av pr create"),
					" or ", colors.CliCmd("av stack submit"), ")\n",
				)
			} else if branch.PullRequest.State == githubv4.PullRequestStateClosed && branch.MergeCommit == "" {
				branch.MergeCommit, err = findMergeCommitWithGitLog(repo, fetchHead, branch)
				if err != nil {
					return nil, errors.Wrap(err, "failed to find the merge commit from git-log")
				}
				if branch.MergeCommit != "" {
					tx.SetBranch(branch)
				}
			}
		}

		if branch.MergeCommit != "" {
			_, _ = fmt.Fprint(os.Stderr,
				"  - skipping sync for merged branch "+
					"(merged in commit ", colors.UserInput(git.ShortSha(branch.MergeCommit)), ")"+
					"\n",
			)
			return nil, nil
		}

		var err error
		cont, err = syncBranchRebase(ctx, repo, tx, opts)
		if err != nil {
			return nil, err
		}
	}

	if cont != nil {
		return cont, nil
	}

	if opts.Push {
		if err := syncBranchPushAndUpdatePullRequest(ctx, repo, client, tx, opts.Branch, pull); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

// syncBranchRebase does the actual rebase part of SyncBranch
func syncBranchRebase(
	ctx context.Context,
	repo *git.Repo,
	tx meta.WriteTx,
	opts SyncBranchOpts,
) (*SyncBranchContinuation, error) {
	branch, _ := tx.Branch(opts.Branch)
	parentState := branch.Parent
	parentBranch, _ := tx.Branch(parentState.Name)
	origParentState := branch.Parent
	origParentBranch := parentBranch
	for parentBranch.MergeCommit != "" {
		parentState = parentBranch.Parent
		parentBranch, _ = tx.Branch(parentState.Name)
	}

	if parentState.Trunk {
		var newUpstreamCommitHash string
		if opts.ToTrunk {
			// First, try to fetch latest commit from the trunk...
			_, _ = fmt.Fprint(
				os.Stderr,
				"  - fetching latest commit from ",
				colors.UserInput("origin/", parentState.Name),
				"\n",
			)
			if _, err := repo.Run(&git.RunOpts{
				Args: []string{"fetch", "origin", parentState.Name},
			}); err != nil {
				_, _ = fmt.Fprint(
					os.Stderr,
					"  - ",
					colors.Failure(
						"error: failed to fetch HEAD of ",
					),
					colors.UserInput(parentState.Name),
					colors.Failure(" from origin: ", err.Error()),
					"\n",
				)
				return nil, errors.WrapIff(
					err,
					"failed to fetch trunk branch %q from origin",
					parentState.Name,
				)
			}

			// NOTE: Strictly speaking, if a user doesn't use the default refspec (e.g. fetch is
			// not +refs/heads/*:refs/remotes/origin/*, the remote tracking branch is not
			// origin/$TRUNK. As we just fetched from a remote, it'd be safe to use FETCH_HEAD.
			trunkHead, err := repo.RevParse(&git.RevParse{Rev: "origin/" + parentState.Name})
			if err != nil {
				return nil, errors.WrapIff(err, "failed to get HEAD of %q", parentState.Name)
			}
			newUpstreamCommitHash = trunkHead
		} else if origParentBranch.MergeCommit != "" {
			newUpstreamCommitHash = origParentBranch.MergeCommit
			// Fetch the merge commit from the remote.
			// This is important to support the sequence where a parent branch
			// in the stack is merged using MergeQueue or GitHub then the user
			// runs `av sync` on a child branch. Assuming they haven't done a
			// git fetch/git pull between, the merge commit will not be in their
			// local repo, and we'll fail to rebase with an error along the
			// lines of "commit abcd1234 does not exist".
			if _, err := repo.Run(&git.RunOpts{
				Args: []string{"fetch", "origin", newUpstreamCommitHash},
			}); err != nil {
				_, _ = fmt.Fprint(
					os.Stderr,
					colors.Failure("  - error: failed to fetch commit "),
					colors.UserInput(git.ShortSha(newUpstreamCommitHash)),
					colors.Failure(" from origin: ", err.Error()),
				)
				return nil, errors.WrapIff(err, "failed to fetch merge commit from origin")
			}
		} else {
			_, _ = fmt.Fprint(os.Stderr,
				"  - branch is a stack root, nothing to do",
				" (run ", colors.CliCmd("av stack sync --trunk"),
				" to sync against the latest commit in ", colors.UserInput(parentState.Name), ")\n",
			)
			return nil, nil
		}

		var origUpstream string
		if origParentState.Trunk {
			// We do not know the original trunk commit hashes. Use the current one as
			// an approximation.
			origUpstream = newUpstreamCommitHash
		} else {
			origUpstream = origParentState.Head
		}

		continuation := SyncBranchContinuation{
			NewParentName: parentState.Name,
		}
		rebase, err := repo.RebaseParse(git.RebaseOpts{
			Branch:   branch.Name,
			Upstream: origUpstream,
			Onto:     newUpstreamCommitHash,
		})
		if err != nil {
			return nil, err
		}
		msgRebaseResult(rebase)
		if rebase.Status == git.RebaseConflict {
			return &continuation, nil
		}
		syncBranchUpdateParent(tx, branch, &continuation)
		return nil, nil
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
	if origParentBranch.MergeCommit != "" {
		short := git.ShortSha(origParentBranch.MergeCommit)
		_, _ = fmt.Fprint(os.Stderr,
			"  - parent ", colors.UserInput(origParentState.Name),
			" (pull ", colors.UserInput("#", origParentBranch.PullRequest.GetNumber()), ")",
			" was merged\n",
		)
		_, _ = fmt.Fprint(os.Stderr,
			"  - rebasing ", colors.UserInput(branch.Name),
			" on top of merge commit ", colors.UserInput(short), "\n",
		)

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
		newUpstreamCommitHash := origParentBranch.MergeCommit
		var origUpstream string
		if origParentState.Trunk {
			var err error
			origUpstream, err = repo.RevParse(&git.RevParse{Rev: "origin/" + origParentState.Name})
			if err != nil {
				return nil, errors.WrapIff(err, "failed to get HEAD of %q", origParentState.Name)
			}
		} else {
			origUpstream = origParentState.Head
		}
		continuation := SyncBranchContinuation{
			NewParentName: parentState.Name,
		}
		if !parentState.Trunk {
			_, _ = fmt.Fprint(
				os.Stderr,
				"  - Parent branch ",
				colors.UserInput(origParentState.Name),
				" was merged into non-trunk branch ",
				colors.UserInput(parentState.Name),
				", reparenting ",
				colors.UserInput(branch.Name),
				" onto ",
				colors.UserInput(parentState.Name),
				"\n",
			)
			continuation.NewParentCommit = newUpstreamCommitHash
		}
		rebase, err := repo.RebaseParse(git.RebaseOpts{
			Branch:   branch.Name,
			Upstream: origUpstream,
			Onto:     newUpstreamCommitHash,
		})
		if err != nil {
			return nil, err
		}
		msgRebaseResult(rebase)
		if rebase.Status == git.RebaseConflict {
			return &continuation, nil
		}
		syncBranchUpdateParent(tx, branch, &continuation)
		return nil, nil
	}

	// Scenario 2: the branch is up-to-date with its parent.
	parentHead, err := repo.RevParse(&git.RevParse{Rev: parentState.Name})
	if err != nil {
		return nil, errors.WrapIff(
			err,
			"failed to resolve HEAD of parent branch %q",
			parentState.Name,
		)
	}
	mergeBase, err := repo.MergeBase(&git.MergeBase{
		Revs: []string{parentHead, branch.Name},
	})
	if err != nil {
		return nil, errors.WrapIff(
			err,
			"failed to compute merge base of %q and %q",
			parentState.Name,
			branch.Name,
		)
	}
	if mergeBase == parentHead {
		_, _ = fmt.Fprint(os.Stderr,
			"  - already up-to-date with parent ", colors.UserInput(parentState.Name),
			"\n",
		)
		continuation := SyncBranchContinuation{
			NewParentName:   parentState.Name,
			NewParentCommit: parentHead,
		}
		syncBranchUpdateParent(tx, branch, &continuation)
		return nil, nil
	}

	// Scenario 3: the branch is not up-to-date with its parent.
	_, _ = fmt.Fprint(os.Stderr,
		"  - syncing branch ", colors.UserInput(branch.Name),
		" on latest commit ", colors.UserInput(git.ShortSha(parentHead)),
		" of parent branch ", colors.UserInput(parentState.Name),
		"\n",
	)
	var origUpstream string
	if origParentState.Trunk {
		// This can happen if the branch is originally a stack root and reparented to
		// another branch (and became non-stack-root).
		var err error
		origUpstream, err = repo.RevParse(&git.RevParse{Rev: "origin/" + origParentState.Name})
		if err != nil {
			return nil, errors.WrapIff(err, "failed to get HEAD of %q", origParentState.Name)
		}
	} else {
		origUpstream = origParentState.Head
	}
	// We need to use `rebase --onto` here and be very careful about how we
	// determine the commits that are being rebased on top of parentHead.
	// Suppose we have a history like
	//     A---B---C---D  main
	//      \
	//       Q---R  stacked-1
	//        \
	//         T  stacked-2
	//          \
	//           W  stacked-3
	// where R is a commit that was added to stacked-1 after stacked-2 was
	// created. After syncing stacked-2 against stacked-1, we have
	//     A---B---C---D  main
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
	continuation := SyncBranchContinuation{
		NewParentName:   parentState.Name,
		NewParentCommit: parentHead,
	}
	rebase, err := repo.RebaseParse(git.RebaseOpts{
		Branch:   branch.Name,
		Upstream: origUpstream,
		Onto:     parentHead,
	})
	if err != nil {
		return nil, err
	}
	msgRebaseResult(rebase)
	if rebase.Status == git.RebaseConflict {
		return &continuation, nil
	}
	syncBranchUpdateParent(tx, branch, &continuation)
	return nil, nil
}

func fetchRemoteTrunkHead(repo *git.Repo, tx meta.WriteTx, branch meta.Branch) (string, error) {
	parent, ok := meta.Trunk(tx, branch.Name)
	if !ok {
		return "", errors.Errorf("failed to find the trunk branch for %q", branch.Name)
	}

	if _, err := repo.Git("fetch", "origin", parent); err != nil {
		return "", errors.WrapIff(err, "failed to fetch %q from origin", parent)
	}
	commitHash, err := repo.RevParse(&git.RevParse{Rev: "FETCH_HEAD"})
	if err != nil {
		return "", errors.WrapIff(err, "failed to read the commit hash of %q", parent)
	}
	return commitHash, nil
}

// findMergeCommitWithGitLog looks for the merge commit for a specified PR.
//
// Usually, GitHub should set which commit closes a pull request. This is known to be not that
// reliable. When we cannot find a merge commit for a closed PR, we try to find if any commit in the
// upstream closes the pull request as a fallback.
func findMergeCommitWithGitLog(
	repo *git.Repo,
	upstreamCommit string,
	branch meta.Branch,
) (string, error) {
	// Find all commits that have been merged into the trunk since this branch
	cis, err := repo.Log(git.LogOpts{RevisionRange: []string{upstreamCommit, "^" + branch.Name}})
	if err != nil {
		return "", err
	}
	closedPRs := git.FindClosesPullRequestComments(cis)
	return closedPRs[branch.PullRequest.Number], nil
}

func syncBranchContinue(
	ctx context.Context,
	repo *git.Repo,
	tx meta.WriteTx,
	opts SyncBranchOpts,
	branch meta.Branch,
) (*SyncBranchContinuation, error) {
	var rebaseOpts git.RebaseOpts
	if opts.Skip {
		rebaseOpts.Skip = true
	} else {
		rebaseOpts.Continue = true
	}
	rebase, err := repo.RebaseParse(rebaseOpts)
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
		return opts.Continuation, nil
	default:
		msgRebaseResult(rebase)
	}

	// Finish setting the parent for the branch
	syncBranchUpdateParent(tx, branch, opts.Continuation)
	return nil, nil
}

func syncBranchUpdateParent(
	tx meta.WriteTx,
	branch meta.Branch,
	continuation *SyncBranchContinuation,
) {
	oldParentState := branch.Parent
	branch.Parent = meta.BranchState{
		Name: continuation.NewParentName,
	}
	if continuation.NewParentCommit == "" {
		branch.Parent.Trunk = true
	} else {
		branch.Parent.Head = continuation.NewParentCommit
	}
	tx.SetBranch(branch)

	if !oldParentState.Trunk && branch.Parent.Trunk {
		_, _ = fmt.Fprint(os.Stderr,
			"  - this branch is now a stack root based on trunk branch ",
			colors.UserInput(branch.Parent.Name), "\n",
		)
	}
}

func syncBranchPushAndUpdatePullRequest(
	ctx context.Context,
	repo *git.Repo,
	client *gh.Client,
	tx meta.WriteTx,
	branchName string,
	// pr can be nil, in which case the PR info is fetched from GitHub
	pr *gh.PullRequest,
) error {
	branch, _ := tx.Branch(branchName)
	if branch.PullRequest == nil || branch.PullRequest.ID == "" {
		return nil
	}

	if pr == nil {
		var err error
		pr, err = client.PullRequest(ctx, branch.PullRequest.ID)
		if err != nil {
			return errors.WrapIff(err, "failed to fetch pull request info for %q", branch.Name)
		}
	}

	if pr.State == githubv4.PullRequestStateClosed || pr.State == githubv4.PullRequestStateMerged {
		_, _ = fmt.Fprint(os.Stderr,
			"  - ", colors.Warning("WARNING:"),
			" pull request ", colors.UserInput("#", pr.Number),
			" is ", colors.UserInput(strings.ToLower(string(pr.State))),
			", skipping push\n",
		)
		_, _ = fmt.Fprint(os.Stderr,
			"      - re-open the pull request (or create a new one with ",
			colors.CliCmd("av pr create"), ") to push changes\n",
		)
		return nil
	}

	rebaseWithDraft := shouldRebaseWithDraft(repo, pr)
	if rebaseWithDraft {
		_, err := client.ConvertPullRequestToDraft(ctx, pr.ID)
		if err != nil {
			return errors.WrapIff(err, "failed to convert pull request to draft")
		}
	}

	if err := Push(repo, branchName, PushOpts{
		Force:                        ForceWithLease,
		SkipIfRemoteBranchNotExist:   true,
		SkipIfRemoteBranchIsUpToDate: true,
	}); err != nil {
		return err
	}

	prMeta, err := getPRMetadata(tx, branch, nil)
	if err != nil {
		return err
	}

	prBody := AddPRMetadataAndStack(pr.Body, prMeta, branchName, nil)
	if _, err := client.UpdatePullRequest(ctx, githubv4.UpdatePullRequestInput{
		PullRequestID: branch.PullRequest.ID,
		BaseRefName:   gh.Ptr(githubv4.String(branch.Parent.Name)),
		Body:          gh.Ptr(githubv4.String(prBody)),
	}); err != nil {
		return err
	}

	if rebaseWithDraft {
		if _, err := client.MarkPullRequestReadyForReview(ctx, pr.ID); err != nil {
			return err
		}
	}

	return nil
}

func shouldRebaseWithDraft(repo *git.Repo, pr *gh.PullRequest) bool {
	if pr.IsDraft {
		// If the PR is already a draft, then we don't need to do anything.
		// This prevents us from accidentally un-drafting a draft PR when we're
		// done rebasing it.
		return false
	}

	if config.Av.PullRequest.RebaseWithDraft == nil {
		if ghutils.HasCodeowners(repo) {
			_, _ = fmt.Fprint(
				os.Stderr,
				"  - converting pull request to draft for rebase to avoid adding unnecessary CODEOWNERS\n",
				"      - set ",
				colors.CliCmd("pullRequest.rebaseWithDraft"),
				" in your configuration file to explicitly control this behavior and to suppress this message\n",
				"      - see https://docs.aviator.co/reference/aviator-cli/configuration#config-option-reference for more information\n",
			)
			return true
		}
		return false
	}

	return *config.Av.PullRequest.RebaseWithDraft
}
