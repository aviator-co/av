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
	"github.com/aviator-co/av/internal/utils/sliceutils"
	"github.com/shurcooL/githubv4"
)

type SyncBranchOpts struct {
	Branch string
	Fetch  bool
	Push   bool
	// If specified, synchronize the branch against the latest version of the
	// trunk branch. This value is ignored if the branch is not a stack root.
	ToTrunk bool
	Skip    bool

	Continuation *SyncBranchContinuation
}

type SyncBranchResult struct {
	git.RebaseResult

	// If set, the sync needs to be continued.
	// This is set if and only if RebaseResult.Status is RebaseConflict
	Continuation *SyncBranchContinuation

	// The updated branch metadata (if the rebase was successful)
	Branch meta.Branch

	// The branch should be deleted.
	DeleteBranch bool
}

type (
	SyncBranchContinuation struct {
		// The original HEAD commit of the branch.
		OldHead string `json:"oldHead"`
	}
)

// SyncBranch synchronizes a branch with its parent.
func SyncBranch(
	ctx context.Context,
	repo *git.Repo,
	client *gh.Client,
	tx meta.WriteTx,
	opts SyncBranchOpts,
) (*SyncBranchResult, error) {
	branch, _ := tx.Branch(opts.Branch)
	_, _ = fmt.Fprint(os.Stderr, "Synchronizing branch ", colors.UserInput(branch.Name), "...\n")

	var res *SyncBranchResult
	var pull *gh.PullRequest

	if opts.Continuation != nil {
		var err error
		res, err = syncBranchContinue(ctx, repo, tx, opts, branch)
		if err != nil {
			return nil, err
		}
	} else {
		if opts.Fetch {
			update, err := UpdatePullRequestState(ctx, client, tx, branch.Name)
			if err != nil {
				_, _ = fmt.Fprint(os.Stderr, colors.Failure("      - error: ", err.Error()), "\n")
				return nil, errors.Wrap(err, "failed to fetch latest PR info")
			}
			pull = update.Pull
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

		var err error
		res, err = syncBranchRebase(ctx, repo, tx, opts, branch)
		if err != nil {
			return nil, err
		}
	}

	branch = res.Branch
	if res.Status == git.RebaseConflict {
		return res, nil
	}

	if branch.PullRequest != nil && branch.PullRequest.State != githubv4.PullRequestStateOpen {
		if lo, err := hasLeftover(repo, branch, opts); err != nil {
			return nil, err
		} else if lo {
			if !opts.ToTrunk {
				_, _ = fmt.Fprint(os.Stderr,
					"  - ", colors.Warning("WARNING: The PR is closed, but without --trunk, we cannot tell if we can safely delete the branch."),
				)
			} else {
				_, _ = fmt.Fprint(os.Stderr,
					"  - ", colors.Warning("WARNING: The PR is closed, but after rebasing, there are still some changes left locally."),
				)
				// TODO(draftcode): Ask what to do with the leftovers. Right now
				// this keeps the leftover to the merged branch.
			}
		} else {
			res.DeleteBranch = true
		}
	} else if opts.Push {
		if err := syncBranchPushAndUpdatePullRequest(ctx, repo, client, tx, branch, pull); err != nil {
			return nil, err
		}
	}

	return res, nil
}

// syncBranchRebase does the actual rebase part of SyncBranch
func syncBranchRebase(
	ctx context.Context,
	repo *git.Repo,
	tx meta.WriteTx,
	opts SyncBranchOpts, branch meta.Branch,
) (*SyncBranchResult, error) {
	branchHead, err := repo.RevParse(&git.RevParse{Rev: branch.Name})
	if err != nil {
		return nil, errors.WrapIff(err, "failed to get head of branch %q", branch.Name)
	}

	rebaseOpt := git.RebaseOpts{
		Branch: opts.Branch,
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
	if branch.IsStackRoot() {
		if opts.ToTrunk {
			trunk := branch.Parent.Name
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

			// NOTE: Strictly speaking, if a user doesn't use the default refspec (e.g.
			// fetch is not +refs/heads/*:refs/remotes/origin/*, the remote tracking
			// branch is not origin/$TRUNK. As we just fetched from a remote, it'd be
			// safe to use FETCH_HEAD.
			trunkHead, err := repo.RevParse(&git.RevParse{Rev: "origin/" + trunk})
			if err != nil {
				return nil, errors.WrapIff(err, "failed to get HEAD of %q", trunk)
			}
			rebaseOpt.Upstream = trunkHead
			rebaseOpt.Onto = trunkHead
		} else if branch.MergeCommit == "" {
			// Do not rebase the stack root unless --trunk or it's merged.
			_, _ = fmt.Fprint(os.Stderr,
				"  - branch is a stack root, nothing to do",
				" (run ", colors.CliCmd("av stack sync --trunk"),
				" to sync against the latest commit in ", colors.UserInput(branch.Parent.Name), ")\n",
			)
			return &SyncBranchResult{
				git.RebaseResult{Status: git.RebaseAlreadyUpToDate},
				nil,
				branch,
				false,
			}, nil
		} else {
			short := git.ShortSha(branch.MergeCommit)
			_, _ = fmt.Fprint(os.Stderr,
				"  - branch ", colors.UserInput(branch.Name),
				" (pull ", colors.UserInput("#", branch.PullRequest.GetNumber()), ")",
				" was merged\n",
			)
			_, _ = fmt.Fprint(os.Stderr,
				"  - rebasing ", colors.UserInput(branch.Name),
				" on top of merge commit ", colors.UserInput(short), "\n",
			)
			if opts.Fetch {
				if _, err := repo.Git("fetch", "origin", branch.MergeCommit); err != nil {
					return nil, errors.WrapIff(err, "failed to fetch merge commit %q from origin", short)
				}
			}
			// Ideally, this makes the branch.Name points to branch.MergeCommit. If the result
			// is different from MergeCommit, it means that there's a leftover change that was
			// not a part of the merged PR.
			rebaseOpt.Upstream = branch.MergeCommit
			rebaseOpt.Onto = branch.MergeCommit
		}
	} else {
		currParentHead, err := repo.RevParse(&git.RevParse{Rev: branch.Parent.Name})
		if err != nil {
			return nil, errors.WrapIff(err, "failed to resolve HEAD of parent branch %q", branch.Parent.Name)
		}
		short := git.ShortSha(currParentHead)
		_, _ = fmt.Fprint(os.Stderr,
			"  - rebasing ", colors.UserInput(branch.Name),
			" on top of parent commit ", colors.UserInput(short), "\n",
		)
		rebaseOpt.Upstream = branch.Parent.Head
		rebaseOpt.Onto = currParentHead
	}

	rebase, err := repo.RebaseParse(rebaseOpt)
	if err != nil {
		return nil, err
	}
	if !branch.Parent.Trunk {
		branch.Parent.Head = rebaseOpt.Onto
	}
	tx.SetBranch(branch)

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
			false,
		}, nil
	default:
		return &SyncBranchResult{
			*rebase,
			nil,
			branch,
			false,
		}, nil
	}
}

func hasLeftover(repo *git.Repo, branch meta.Branch, opts SyncBranchOpts) (bool, error) {
	// The PR is merged. We get the rebase upstream (more strictly speaking the --onto
	// part) and check if currently the branch points to the same commit. If they are
	// pointing to the same one, no leftover.
	var rebaseUpstream string
	if branch.IsStackRoot() {
		if opts.ToTrunk {
			trunk := branch.Parent.Name
			trunkHead, err := repo.RevParse(&git.RevParse{Rev: "origin/" + trunk})
			if err != nil {
				return false, errors.WrapIff(err, "failed to get HEAD of %q", trunk)
			}
			rebaseUpstream = trunkHead
		} else if branch.MergeCommit == "" {
			// The PR is closed without merge. There are two possibilities (1) the PR is
			// effectively merged by other ways (e.g. MergeQueue's fast-forward mode) or
			// (2) the PR is closed by a human. Either way, in order to tell if there's
			// a leftover, we need to rebase on top of the current trunk, which requires
			// --trunk. In this case, --trunk is not specified, so we cannot tell if
			// there's a leftover.
			return true, nil
		} else {
			rebaseUpstream = branch.MergeCommit
		}
	} else {
		parentHead, err := repo.RevParse(&git.RevParse{Rev: branch.Parent.Name})
		if err != nil {
			return false, errors.WrapIff(err, "failed to get HEAD of %q", branch.Parent.Name)
		}
		rebaseUpstream = parentHead
	}
	currentHead, err := repo.RevParse(&git.RevParse{Rev: branch.Name})
	if err != nil {
		return false, errors.WrapIff(err, "failed to get HEAD of %q", branch.Name)
	}
	return currentHead != rebaseUpstream, nil
}

func syncBranchContinue(
	ctx context.Context,
	repo *git.Repo,
	tx meta.WriteTx,
	opts SyncBranchOpts,
	branch meta.Branch,
) (*SyncBranchResult, error) {
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
		return &SyncBranchResult{*rebase, opts.Continuation, branch, false}, nil
	default:
		msgRebaseResult(rebase)
	}

	return &SyncBranchResult{*rebase, nil, branch, false}, nil
}

func syncBranchUpdateNewTrunk(
	tx meta.WriteTx,
	branch meta.Branch,
	newTrunk string,
) (meta.Branch, error) {
	oldParent, _ := tx.Branch(branch.Parent.Name)
	var err error

	branch.Parent = meta.BranchState{
		Name:  newTrunk,
		Trunk: true,
	}
	if err != nil {
		return branch, err
	}
	_, _ = fmt.Fprint(os.Stderr,
		"  - this branch is now a stack root based on trunk branch ",
		colors.UserInput(branch.Parent.Name), "\n",
	)
	tx.SetBranch(branch)

	// Remove from the old parent branches metadata
	if len(oldParent.Children) > 0 {
		oldParent.Children = sliceutils.DeleteElement(oldParent.Children, branch.Name)
		tx.SetBranch(oldParent)
	}

	return branch, nil
}

func syncBranchPushAndUpdatePullRequest(
	ctx context.Context,
	repo *git.Repo,
	client *gh.Client,
	tx meta.WriteTx,
	branch meta.Branch,
	// pr can be nil, in which case the PR info is fetched from GitHub
	pr *gh.PullRequest,
) error {
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

	if err := Push(repo, PushOpts{
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
	prBody := AddPRMetadata(pr.Body, prMeta)
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
			_, _ = fmt.Fprint(os.Stderr,
				"  - converting pull request to draft for rebase to avoid adding unnecessary CODEOWNERS\n",
				"      - set ", colors.CliCmd("pullRequest.rebaseWithDraft"), " in your configuration file to explicitly control this behavior and to suppress this message\n",
				"      - see https://docs.aviator.co/reference/aviator-cli/configuration#config-option-reference for more information\n",
			)
			return true
		}
		return false
	}

	return *config.Av.PullRequest.RebaseWithDraft
}
