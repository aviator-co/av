package main

import (
	"emperror.dev/errors"
	"encoding/json"
	"fmt"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/stacks"
	"github.com/aviator-co/av/internal/utils/stringutils"
	"github.com/kr/text"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"path"
	"strings"
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
}

// stackSyncState is the state of an in-progress sync operation.
// It is written to a file if the sync is interrupted (so it can be resumed with
// the --continue flag).
type stackSyncState struct {
	// The branch to return to when the sync is complete.
	OriginalBranch string `json:"originalBranch"`
	// The current branch being synced.
	CurrentBranch string `json:"currentBranch"`
	// If set, we need to continue the current sync step before syncing the
	// remaining branches. Not serialized to JSON because it's only set by the
	// --continue flag.
	Continue bool            `json:"-"`
	Config   stackSyncConfig `json:"config"`
}

var stackSyncFlags struct {
	// Include all the options from stackSyncConfig
	stackSyncConfig
	// If set, we're continuing a previous sync.
	Continue bool
	// If set, abort an in-progress sync operation.
	Abort bool
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
		if stackSyncFlags.Continue && stackSyncFlags.Abort {
			return errors.New("cannot use --continue and --abort together")
		}

		// Check for unstaged changes
		repo, err := getRepo()
		if err != nil {
			return err
		}
		diff, err := repo.Diff(&git.DiffOpts{Quiet: true})
		if err != nil {
			return err
		}
		if !diff.Empty {
			return errors.New("refusing to sync: there are unstaged changes in the working tree (use `git add` to stage changes)")
		}

		// Read any pre-existing state.
		// This is required to allow us to handle --continue/--abort
		state, err := readStackSyncState(repo)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		// TODO[mvp]:
		//     Let's make sure we have a reasonable story around what happens in
		//     edge cases. When we relinquish control of the repo back to the
		//     user, they might do wild things (checkout a different branch,
		//     run the continue seventeen days and seventy seven commits later,
		//     etc.).
		if stackSyncFlags.Abort {
			if state.CurrentBranch == "" {
				return errors.New("no sync in progress")
			}
			err := writeStackSyncState(repo, nil)
			if err != nil {
				return errors.Wrap(err, "failed to reset stack sync state")
			}
			_, _ = fmt.Fprintf(os.Stderr, "Aborted stack sync for branch %q\n", state.CurrentBranch)
			return nil
		} else if stackSyncFlags.Continue {
			if state.CurrentBranch == "" {
				return errors.New("no sync in progress")
			}
			state.Continue = true
		} else {
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
		}

		// Set the original branch so we can return to it when the sync is done
		if state.OriginalBranch == "" {
			state.OriginalBranch = state.CurrentBranch
		}

		// Construct the list of branches we need to sync
		branches, err := meta.ReadAllBranches(repo)
		if err != nil {
			return err
		}
		var branchesToSync []string
		if state.Continue || state.Config.Current {
			// If we're continuing, we assume the previous branches are already
			// synced correctly and we just need to sync the subsequent
			// branches. (This matters because if we're here, that means there
			// was a sync conflict, and we need to `git rebase --continue`
			// before we can sync the next branch, and git will scream at us if
			// we try to do something in the repo before we finish that)
			branchesToSync = []string{state.CurrentBranch}
		} else {
			// Otherwise, this is not a --continue, so we want to sync every
			// ancestor first.
			currentBranch, err := repo.CurrentBranchName()
			if err != nil {
				return err
			}
			branchesToSync, err = previousBranches(branches, currentBranch)
			if err != nil {
				return err
			}
			branchesToSync = append(branchesToSync, currentBranch)
		}
		// Either way (--continue or not), we sync all subsequent branches.
		nextBranches, err := subsequentBranches(branches, branchesToSync[len(branchesToSync)-1])
		if err != nil {
			return err
		}
		branchesToSync = append(branchesToSync, nextBranches...)

		logrus.WithField("branches", branchesToSync).Debug("determined branches to sync")
		var resErr error
	loop:
		for _, currentBranch := range branchesToSync {
			state.CurrentBranch = currentBranch
			currentMeta, ok := branches[currentBranch]
			if !ok {
				return errors.Errorf("stack metadata not found for branch %q", currentBranch)
			}
			if currentMeta.Parent == "" {
				continue
			}
			log := logrus.WithFields(logrus.Fields{
				"current": currentBranch,
				"parent":  currentMeta.Parent,
			})

			// Checkout the branch (unless we need to continue a rebase, in which
			// case Git will yell at us)
			var res *stacks.SyncResult
			if state.Continue {
				log.Debug("finishing previous interrupted sync...")
				res, err = stacks.SyncContinue(repo, stacks.StrategyRebase)

				// Only the first branch needs to be --continue'd, for the rest
				// we just do a normal merge/rebase
				state.Continue = false
			} else {
				log.Debug("syncing branch...")
				_, err := repo.CheckoutBranch(&git.CheckoutBranch{Name: currentBranch})
				if err != nil {
					return err
				}
				res, err = stacks.SyncBranch(repo, &stacks.SyncBranchOpts{
					Parent:   currentMeta.Parent,
					Strategy: stacks.StrategyRebase,
				})
			}
			log.WithField("result", res).WithError(err).Debug("sync finished")
			if err != nil {
				return errors.WrapIff(err, "failed to sync branch %q", currentBranch)
			}

			fmt.Printf("Syncing %q into %q: ", currentMeta.Parent, currentBranch)
			switch res.Status {
			case stacks.SyncAlreadyUpToDate:
				fmt.Println("already up-to-date (no-op)")
			case stacks.SyncUpdated:
				fmt.Println("updated")
			case stacks.SyncConflict:
				fmt.Println("conflict")
				if res.Hint != "" {
					// Remove the "hint: ..." lines from the output since they
					// contain instructions that tell the user to run
					// `git rebase --continue` which we actually *don't* want
					// them to do.
					hint := stringutils.RemoveLines(res.Hint, "hint: ")
					_, _ = fmt.Println(text.Indent(hint, "    "))
				}
				resErr = errors.Errorf("conflict detected: please resolve and then run `av stack sync --continue`")
				break loop
			case stacks.SyncNotInProgress:
				fmt.Println("invalid state")
				// TODO:
				// 		Would be nice to have some way to show more details than
				// 		this, but having multi-line error's is not very idiomatic
				//		with go. A future improvement might be having an
				//		interface like ErrorDetails that can be used to show
				//		help text if it's the return error from a CLI
				//		invocation.
				// Note:
				//		We don't just auto-abort here because it's unclear what
				//		the actual state is here. We'd rather err on the side of
				// 		making the user be explicit than do something unexpected
				//		with their code/repository.
				resErr = errors.Errorf("rebase was completed or cancelled outside of av: please run `av stack sync --abort` to abort the current sync and then retry")
			default:
				logrus.Panicf("invariant error: unknown sync result: %v", res)
			}
		}

		// TODO:
		// 		this weird thing where we set resErr then break outside of the
		//		loop is a code smell which probably indicates we should have
		//		another function wrapping a lot of the logic above, but we'll
		//		fix that at some point
		if resErr != nil {
			if err := writeStackSyncState(repo, &state); err != nil {
				logrus.WithError(resErr).Warn("while handling error, failed to write stack sync state")
				return errors.Wrap(err, "failed to write stack sync state")
			}
			return resErr
		}

		if err := writeStackSyncState(repo, nil); err != nil {
			return errors.Wrap(err, "failed to reset stack sync state")
		}

		_, _ = fmt.Fprintf(os.Stderr, "Stack sync complete\n")

		// Return to the starting branch when we're done
		if _, err := repo.CheckoutBranch(&git.CheckoutBranch{
			Name: state.OriginalBranch,
		}); err != nil {
			return err
		}

		return nil
	},
}

type syncResult struct {
	Conflict      bool
	CurrentBranch string
}

// Find all the ancestor branches of the given branch name and append them to
// the given slice (in topological order: a comes before b if a is an ancestor
// of b).
func previousBranches(branches map[string]meta.Branch, name string) ([]string, error) {
	current, ok := branches[name]
	if !ok {
		return nil, errors.Errorf("branch metadata not found for %q", name)
	}
	if current.Parent == "" {
		return nil, nil
	}
	if current.Parent == name {
		logrus.Fatalf("invariant error: branch %q is its own parent (this is probably a bug with av)", name)
	}
	previous, err := previousBranches(branches, current.Parent)
	if err != nil {
		return nil, err
	}
	return append(previous, current.Parent), nil
}

func subsequentBranches(branches map[string]meta.Branch, name string) ([]string, error) {
	logrus.Debugf("finding subsequent branches for %q", name)
	var res []string
	branchMeta, ok := branches[name]
	if !ok {
		return nil, fmt.Errorf("branch metadata not found for %q", name)
	}
	if len(branchMeta.Children) == 0 {
		return res, nil
	}
	for _, child := range branchMeta.Children {
		res = append(res, child)
		desc, err := subsequentBranches(branches, child)
		if err != nil {
			return nil, err
		}
		res = append(res, desc...)
	}

	return res, nil
}

const stackSyncStateFile = "stack-sync.state.json"

func readStackSyncState(repo *git.Repo) (stackSyncState, error) {
	var state stackSyncState
	data, err := ioutil.ReadFile(path.Join(repo.GitDir(), "av", stackSyncStateFile))
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, err
	}
	return state, nil
}

func writeStackSyncState(repo *git.Repo, state *stackSyncState) error {
	avDir := path.Join(repo.GitDir(), "av")
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
	return ioutil.WriteFile(path.Join(avDir, stackSyncStateFile), data, 0644)
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
}
