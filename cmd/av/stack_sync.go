package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/stringutils"
	"github.com/kr/text"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

// stackSyncConfig contains the configuration for a sync operation.
// It is serializable to JSON to handle the case where the sync is interrupted
// by a merge conflict (so it can be resumed with the --continue flag).
type stackSyncConfig struct {
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
}

// stackSyncState is the state of an in-progress sync operation.
// It is written to a file if the sync is interrupted (so it can be resumed with
// the --continue flag).
type stackSyncState struct {
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
	Continuation *actions.SyncBranchContinuation `json:"continuation,omitempty"`
	// The config of the sync.
	Config stackSyncConfig `json:"config"`
}

var stackSyncFlags struct {
	// Include all the options from stackSyncConfig
	stackSyncConfig
	// If set, we're continuing a previous sync.
	Continue bool
	// If set, abort an in-progress sync operation.
	Abort bool
	// If set, skip a commit and continue a previous sync.
	Skip bool
}

var stackSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "synchronize stacked branches",
	Long: strings.TrimSpace(`
Synchronize stacked branches to be up-to-date with their parent branches.

By default, this command will sync all branches starting at the root of the
stack and recursively rebasing each branch based on the latest commit from the
parent branch.

If the --current flag is given, this command will not recursively sync dependent
branches of the current branch within the stack. This allows you to make changes
to the current branch before syncing the rest of the stack.

If the --trunk flag is given, this command will synchronize changes from the
latest commit to the repository base branch (e.g., main or master) into the
stack. This is useful for rebasing a whole stack on the latest changes from the
base branch.
`),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Argument validation
		if countBools(stackSyncFlags.Continue, stackSyncFlags.Abort, stackSyncFlags.Skip) > 1 {
			return errors.New("cannot use --continue, --abort, and --skip together")
		}
		if stackSyncFlags.Current && stackSyncFlags.Trunk {
			return errors.New("cannot use --current and --trunk together")
		}
		if stackSyncFlags.Parent != "" && stackSyncFlags.Trunk {
			return errors.New("cannot use --parent and --trunk together")
		}

		ctx := context.Background()

		repo, err := getRepo()
		if err != nil {
			return err
		}
		db, err := getDB(repo)
		if err != nil {
			return err
		}
		tx := db.WriteTx()
		defer tx.Abort()

		// Read any preexisting state.
		// This is required to allow us to handle --continue/--abort/--skip
		state, err := readStackSyncState(repo)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		if stackSyncFlags.Abort {
			if state.CurrentBranch == "" || state.Continuation == nil {
				// Try to clear the state file if it exists just to be safe.
				_ = writeStackSyncState(repo, nil)
				return errors.New("no sync in progress")
			}

			// Abort the rebase if we need to
			if stat, _ := os.Stat(path.Join(repo.GitDir(), "REBASE_HEAD")); stat != nil {
				if _, err := repo.Rebase(git.RebaseOpts{Abort: true}); err != nil {
					return errors.WrapIf(err, "failed to abort in-progress rebase")
				}
			}

			err := writeStackSyncState(repo, nil)
			if err != nil {
				return errors.Wrap(err, "failed to reset stack sync state")
			}
			if _, err := repo.CheckoutBranch(&git.CheckoutBranch{Name: state.OriginalBranch}); err != nil {
				return errors.Wrap(err, "failed to checkout original branch")
			}
			_, _ = fmt.Fprintf(os.Stderr, "Aborted stack sync for branch %q\n", state.CurrentBranch)
			return nil
		}

		if !stackSyncFlags.Skip {
			// Make sure all changes are staged unless --skip. git rebase --skip will
			// clean up the changes.
			diff, err := repo.Diff(&git.DiffOpts{Quiet: true})
			if err != nil {
				return err
			}
			if !diff.Empty {
				return errors.New("refusing to sync: there are unstaged changes in the working tree (use `git add` to stage changes)")
			}
		}

		if stackSyncFlags.Continue || stackSyncFlags.Skip {
			if state.CurrentBranch == "" {
				return errors.New("no sync in progress")
			}
		} else {
			// Not a --continue/--skip, we're trying to start a new sync from scratch.
			if state.CurrentBranch != "" {
				return errors.New("a sync is already in progress: use --continue or --abort")
			}

			// NOTE: We have to read the current branch name from the stored
			// state if we're continuing a sync (the case above) because it's
			// likely that we'll be in a detached-HEAD state due to a rebase
			// conflict (and this command will not work).
			// Since we're *not* continuing a sync, we assume we're not in
			// detached HEAD and so this is a reasonable thing to do.
			var err error
			state.CurrentBranch, err = repo.CurrentBranchName()
			if err != nil {
				return err
			}

			state.OriginalBranch = state.CurrentBranch
			state.Config = stackSyncConfig{
				stackSyncFlags.Current,
				stackSyncFlags.Trunk,
				stackSyncFlags.NoPush,
				stackSyncFlags.NoFetch,
				stackSyncFlags.Parent,
			}
		}

		// If we're doing a reparent, that needs to happen first.
		// After that, it's just a normal sync for all of the children branches
		// of the newly-reparented current branch.
		if state.Config.Parent != "" {
			var res *actions.ReparentResult
			var err error
			defaultBranch, err := repo.DefaultBranch()
			if err != nil {
				return err
			}
			opts := actions.ReparentOpts{
				Branch:         state.CurrentBranch,
				NewParent:      state.Config.Parent,
				NewParentTrunk: state.Config.Parent == defaultBranch,
			}
			if stackSyncFlags.Continue || stackSyncFlags.Skip {
				res, err = actions.ReparentSkipContinue(repo, tx, opts, stackSyncFlags.Skip)
			} else {
				res, err = actions.Reparent(repo, tx, opts)
			}
			if err != nil {
				return err
			}
			if !res.Success {
				if err := writeStackSyncState(repo, &state); err != nil {
					return errors.Wrap(err, "failed to write stack sync state")
				}
				_, _ = fmt.Fprint(os.Stderr,
					"Failed to re-parent branch: resolve the conflicts and continue the sync with ",
					colors.CliCmd("av stack sync --continue"),
					"\n",
				)
				hint := stringutils.RemoveLines(res.Hint, "hint: ")
				_, _ = fmt.Fprint(os.Stderr,
					"hint:\n",
					text.Indent(hint, "    "),
					"\n",
				)
				if err := tx.Commit(); err != nil {
					return err
				}
				return nil
			}

			// We're done with the reparenting process, so set this to zero so that
			// we won't try to reparent again later if we have to do a --continue.
			state.Config.Parent = ""
		}

		// For a trunk sync, we need to rebase the stack root against the HEAD
		// of the trunk branch. After that, it's just a normal sync.
		var branchesToSync []string
		if len(state.Branches) != 0 {
			// This is a --continue, so we need to sync the current branch and
			// everything after it.
			currentIdx := slices.Index(state.Branches, state.CurrentBranch)
			if currentIdx == -1 {
				return errors.Errorf(
					"INTERNAL INVARIANT ERROR: current branch %q not found in list of branches for current sync",
					state.CurrentBranch,
				)
			}
			branchesToSync = state.Branches[currentIdx:]
		} else if state.Config.Current {
			// If we're continuing, we assume the previous branches are already
			// synced correctly and we just need to sync the subsequent
			// branches. (This matters because if we're here, that means there
			// was a sync conflict, and we need to `git rebase --continue`
			// before we can sync the next branch, and git will scream at us if
			// we try to do something in the repo before we finish that)
			branchesToSync = []string{state.CurrentBranch}
			state.Branches = branchesToSync
		} else {
			currentBranch, err := repo.CurrentBranchName()
			if err != nil {
				return err
			}
			branchesToSync, err = meta.PreviousBranches(tx, currentBranch)
			if err != nil {
				return err
			}
			branchesToSync = append(branchesToSync, currentBranch)
			nextBranches, err := meta.SubsequentBranches(tx, branchesToSync[len(branchesToSync)-1])
			if err != nil {
				return err
			}
			branchesToSync = append(branchesToSync, nextBranches...)
			state.Branches = branchesToSync
		}
		// Either way (--continue or not), we sync all subsequent branches

		logrus.WithField("branches", branchesToSync).Debug("determined branches to sync")
		//var resErr error
		client, err := getClient(config.Av.GitHub.Token)
		if err != nil {
			return err
		}
		for i, currentBranch := range branchesToSync {
			if i > 0 {
				// Add spacing in the output between each branch sync
				_, _ = fmt.Fprint(os.Stderr, "\n\n")
			}
			state.CurrentBranch = currentBranch
			cont, err := actions.SyncBranch(ctx, repo, client, tx, actions.SyncBranchOpts{
				Branch:       currentBranch,
				Fetch:        !state.Config.NoFetch,
				Push:         !state.Config.NoPush,
				Continuation: state.Continuation,
				ToTrunk:      state.Config.Trunk,
				Skip:         stackSyncFlags.Skip,
			})
			if err != nil {
				return err
			}
			if cont != nil {
				state.Continuation = cont
				if err := writeStackSyncState(repo, &state); err != nil {
					return errors.Wrap(err, "failed to write stack sync state")
				}
				if err := tx.Commit(); err != nil {
					return err
				}
				return errExitSilently{1}
			}

			state.Continuation = nil
		}

		// Return to the original branch
		if _, err := repo.CheckoutBranch(&git.CheckoutBranch{Name: state.OriginalBranch}); err != nil {
			return err
		}
		if err := writeStackSyncState(repo, nil); err != nil {
			return errors.Wrap(err, "failed to write stack sync state")
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return nil
	},
}

const stackSyncStateFile = "stack-sync.state.json"

func readStackSyncState(repo *git.Repo) (stackSyncState, error) {
	var state stackSyncState
	data, err := os.ReadFile(path.Join(repo.AvDir(), stackSyncStateFile))
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, err
	}
	return state, nil
}

func writeStackSyncState(repo *git.Repo, state *stackSyncState) error {
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

func init() {
	stackSyncCmd.Flags().BoolVar(
		&stackSyncFlags.Current, "current", false,
		"only sync changes to the current branch\n(don't recurse into descendant branches)",
	)
	stackSyncCmd.Flags().BoolVar(
		&stackSyncFlags.NoPush, "no-push", false,
		"do not force-push updated branches to GitHub",
	)
	stackSyncCmd.Flags().BoolVar(
		&stackSyncFlags.NoFetch, "no-fetch", false,
		"do not fetch latest PR information from GitHub",
	)
	// TODO[mvp]: better name (--to-trunk?)
	stackSyncCmd.Flags().BoolVar(
		&stackSyncFlags.Trunk, "trunk", false,
		"synchronize the trunk into the stack",
	)
	stackSyncCmd.Flags().BoolVar(
		&stackSyncFlags.Continue, "continue", false,
		"continue an in-progress sync",
	)
	stackSyncCmd.Flags().BoolVar(
		&stackSyncFlags.Abort, "abort", false,
		"abort an in-progress sync",
	)
	stackSyncCmd.Flags().BoolVar(
		&stackSyncFlags.Skip, "skip", false,
		"skip the current commit and continue an in-progress sync",
	)
	stackSyncCmd.Flags().StringVar(
		&stackSyncFlags.Parent, "parent", "",
		"parent branch to rebase onto",
	)
}

func countBools(bs ...bool) int {
	var ret int
	for _, b := range bs {
		if b {
			ret += 1
		}
	}
	return ret
}
