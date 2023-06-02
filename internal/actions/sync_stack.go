package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
)

// StackSyncConfig contains the configuration for a sync operation.
// It is serializable to JSON to handle the case where the sync is interrupted
// by a merge conflict (so it can be resumed with the --continue flag).
type StackSyncConfig struct {
	// If set, only sync up to the current branch (do not sync descendants).
	// This is useful for syncing changes from a parent branch in case the
	// current branch needs to be updated before continuing the sync.
	Current bool `json:"current"`
	// If set, incorporate changes from the trunk (repo base branch) into the stack.
	// Only valid if synchronizing the root of a stack.
	// This effectively re-roots the stack on the latest commit from the trunk.
	Trunk bool `json:"trunk"`
	// If set, do not push to GitHub.
	NoPush bool `json:"noPush"`
	// If set, do not fetch updated PR information from GitHub.
	NoFetch bool `json:"noFetch"`
	// The new parent branch to sync the current branch to.
	Parent string `json:"parent"`
	// If set, delete the merged branches.
	Prune bool `json:"prune"`
}

// StackSyncState is the state of an in-progress sync operation.
// It is written to a file if the sync is interrupted (so it can be resumed with
// the --continue flag).
type StackSyncState struct {
	// The branch to return to when the sync is complete.
	OriginalBranch string `json:"originalBranch"`
	// The branch that's currently being synced.
	CurrentBranch string `json:"currentBranch"`
	// All of the branches that are being synced (including branches that have
	// already been synced).
	// TODO: We should probably store the original HEAD commit for each branch
	//       and revert each branch individually if we --abort.
	Branches []string `json:"branches"`
	// The continuation state for the current branch.
	Continuation *SyncBranchContinuation `json:"continuation,omitempty"`
	// The config of the sync.
	Config StackSyncConfig `json:"config"`
}

type (
	SyncStackOpt  func(*syncStackOpts)
	syncStackOpts struct {
		skipNextCommit bool
	}
)

func WithSkipNextCommit() SyncStackOpt {
	return func(opts *syncStackOpts) {
		opts.skipNextCommit = true
	}
}

// SyncStack performs stack sync on all branches in branchesToSync.
func SyncStack(ctx context.Context,
	repo *git.Repo,
	client *gh.Client,
	tx meta.WriteTx,
	branchesToSync []string,
	state StackSyncState,
	optFns ...SyncStackOpt,
) error {
	opts := &syncStackOpts{}
	for _, optFn := range optFns {
		optFn(opts)
	}

	state.Branches = branchesToSync
	skip := opts.skipNextCommit
	for i, currentBranch := range branchesToSync {
		if i > 0 {
			// Add spacing in the output between each branch sync
			_, _ = fmt.Fprint(os.Stderr, "\n\n")
		}
		state.CurrentBranch = currentBranch
		cont, err := SyncBranch(ctx, repo, client, tx, SyncBranchOpts{
			Branch:       currentBranch,
			Fetch:        !state.Config.NoFetch,
			Push:         !state.Config.NoPush,
			Continuation: state.Continuation,
			ToTrunk:      state.Config.Trunk,
			Skip:         skip,
		})
		if err != nil {
			return err
		}
		if cont != nil {
			state.Continuation = cont
			if err := WriteStackSyncState(repo, &state); err != nil {
				return errors.Wrap(err, "failed to write stack sync state")
			}
			if err := tx.Commit(); err != nil {
				return err
			}
			return ErrExitSilently{ExitCode: 1}
		}
		state.Continuation = nil
		// If skip was specified, it was because the sync was interrupted by a
		// conflict. The user wanted to skip a commit and continue the sync. If
		// we get here, the rebase succeeded, and it doesn't make sense to start
		// subsequent rebases with `git rebase --skip`.
		skip = false
	}

	if state.Config.Prune {
		remoteBranches, err := repo.LsRemote("origin")
		if err != nil {
			return err
		}
		for _, currentBranch := range branchesToSync {
			br, _ := tx.Branch(currentBranch)
			if br.MergeCommit == "" {
				continue
			}
			if len(meta.Children(tx, currentBranch)) > 0 {
				_, _ = fmt.Fprint(
					os.Stderr,
					"  - not deleting merged branch ",
					colors.UserInput(currentBranch),
					" because it still has children",
					"\n",
				)
				continue
			}
			if br.PullRequest == nil {
				_, _ = fmt.Fprint(
					os.Stderr,
					"  - not deleting merged branch ",
					colors.UserInput(currentBranch),
					" because we cannot find the associated pull-request",
					"\n",
				)
				continue
			}
			ref := fmt.Sprintf("refs/pull/%d/head", br.PullRequest.Number)
			remoteHash, ok := remoteBranches[ref]
			if !ok {
				_, _ = fmt.Fprint(
					os.Stderr,
					"  - not deleting merged branch ",
					colors.UserInput(currentBranch),
					" because we cannot find the HEAD of the pull-request",
					"\n",
				)
				continue
			}
			currentHash, err := repo.RevParse(&git.RevParse{Rev: currentBranch})
			if err != nil {
				return errors.Errorf(
					"cannot get the current commit hash of %q: %v",
					currentBranch,
					err,
				)
			}
			if remoteHash != currentHash {
				_, _ = fmt.Fprint(
					os.Stderr,
					"  - not deleting merged branch ",
					colors.UserInput(currentBranch),
					" because the local branch points to a different commit than the merged pull-request",
					"\n",
				)
				continue
			}
			_, _ = fmt.Fprint(os.Stderr,
				"  - deleting merged branch ", colors.UserInput(currentBranch),
				"\n",
			)
			if _, err := repo.Git("switch", "--detach"); err != nil {
				return errors.Errorf("cannot switch to detached HEAD: %v", err)
			}
			if _, err := repo.Git("branch", "-D", currentBranch); err != nil {
				return errors.Errorf("cannot delete merged branch %q: %v", currentBranch, err)
			}
			// There's no children, but this can have a non-trunk parent.
			tx.DeleteBranch(currentBranch)
			if currentBranch == state.OriginalBranch {
				// The original branch is deleted.
				state.OriginalBranch = ""
			}
		}
	}

	// Return to the original branch
	if state.OriginalBranch != "" {
		if _, err := repo.CheckoutBranch(&git.CheckoutBranch{Name: state.OriginalBranch}); err != nil {
			return err
		}
	}
	if err := WriteStackSyncState(repo, nil); err != nil {
		return errors.Wrap(err, "failed to write stack sync state")
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

const stackSyncStateFile = "stack-sync.state.json"

func ReadStackSyncState(repo *git.Repo) (StackSyncState, error) {
	var state StackSyncState
	data, err := os.ReadFile(path.Join(repo.AvDir(), stackSyncStateFile))
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, err
	}
	return state, nil
}

func WriteStackSyncState(repo *git.Repo, state *StackSyncState) error {
	avDir := repo.AvDir()
	if _, err := os.Stat(avDir); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.Mkdir(avDir, 0755); err != nil {
			return err
		}
	}

	// delete the file if state is nil (i.e., --abort)
	if state == nil {
		err := os.Remove(path.Join(avDir, stackSyncStateFile))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	// otherwise, create/write the file
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(avDir, stackSyncStateFile), data, 0644)
}
