package main

import (
	"emperror.dev/errors"
	"encoding/json"
	"fmt"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/stacks"
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
	// Set the parent of the current branch to this branch.
	// This effectively re-roots the stack on a new parent (e.g., adds a branch
	// to the stack).
	Parent string `json:"parent"`
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
	// If set, we need to continue the current sync step before syncing the
	// remaining branches.
	// Not serialized to JSON because it's only set by the --continue flag.
	Continue bool `json:"-"`
}

// stackSyncState is the state of an in-progress sync operation.
// It is written to a file if the sync is interrupted (so it can be resumed with
// the --continue flag).
type stackSyncState struct {
	CurrentBranch string          `json:"currentBranch"`
	Config        stackSyncConfig `json:"config"`
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
		repo, err := getRepo()
		if err != nil {
			return err
		}

		diff, err := repo.Diff(&git.DiffOpts{Quiet: true})
		if err != nil {
			return err
		}
		if !diff.Empty {
			return errors.New("refusing to sync: there are unstaged changes in the working tree")
		}
		logrus.Debugf("%#+v", diff)

		config := stackSyncFlags.stackSyncConfig

		// check for --continue/--abort
		// TODO[mvp]:
		//     Let's make sure we have a reasonable story around what happens in
		//     edge cases. When we relinquish control of the repo back to the
		//     user, they might do wild things (checkout a different branch,
		//     run the continue seventeen days and seventy seven commits later,
		//     etc.).
		if stackSyncFlags.Continue && stackSyncFlags.Abort {
			return errors.New("cannot use --continue and --abort together")
		}
		existingState, err := readStackSyncState(repo)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if stackSyncFlags.Abort {
			if existingState.CurrentBranch == "" {
				return errors.New("no sync in progress")
			}
			err := writeStackSyncState(repo, nil)
			if err != nil {
				return errors.Wrap(err, "failed to reset stack sync state")
			}
			_, _ = fmt.Fprintf(os.Stderr, "Aborted stack sync for branch %q\n", existingState.CurrentBranch)
			return nil
		} else if stackSyncFlags.Continue {
			if existingState.CurrentBranch == "" {
				return errors.New("no sync in progress")
			}
			config = existingState.Config
			config.Continue = true
		} else {
			if existingState.CurrentBranch != "" {
				return errors.New("a sync is already in progress: use --continue or --abort")
			}
		}

		conflict, err := doStackSync(repo, config)
		if err != nil {
			return err
		}
		if conflict {
			currentBranch, err := repo.CurrentBranchName()
			if err != nil {
				return err
			}
			if err := writeStackSyncState(repo, &stackSyncState{
				CurrentBranch: currentBranch,
				Config:        config,
			}); err != nil {
				return errors.Wrap(err, "failed to write stack sync state")
			}
			return errors.New("conflict detected: please resolve and then run `av stack sync --continue`")
		}

		if err := writeStackSyncState(repo, nil); err != nil {
			return errors.Wrap(err, "failed to reset stack sync state")
		}

		_, _ = fmt.Fprintf(os.Stderr, "Stack sync complete\n")
		return nil
	},
}

// doStackSync performs the stack sync operation.
// It returns a boolean indicating whether or not the sync was interrupted
// (if true, the stack sync can be resumed with the --continue flag).
// TODO[mvp]:
//    This whole function is kind of a hot mess, and a lot of it is rooted in
//    how awkward the API is for dealing with "branch trees" (terminology is
//    hard when you're creating a meta-tree-structure on top of git branches).
//    Let's refactor that a bit and then re-work this.
func doStackSync(repo *git.Repo, config stackSyncConfig) (bool, error) {
	root, err := stacks.GetCurrentRoot(repo)
	if err != nil {
		return false, err
	}

	currentBranch, err := repo.CurrentBranchName()
	if err != nil {
		return false, err
	}

	currentTree := root
	for {
		if currentTree.Branch.Name == currentBranch {
			break
		}
		if len(currentTree.Next) > 1 {
			return false, errors.Errorf("unsupported: branch %q has multiple stack children", currentTree.Branch.Name)
		}
		if len(currentTree.Next) == 0 {
			return false, errors.Errorf("invariant error: couldn't find branch %q in stack", currentBranch)
		}
		currentTree = currentTree.Next[0]
	}

	for {
		res, err := stacks.SyncBranch(repo, &stacks.SyncBranchOpts{
			Parent:   currentTree.Branch.Parent,
			Continue: config.Continue,
		})
		if err != nil {
			return false, errors.WrapIff(err, "failed to sync branch %q", currentBranch)
		}
		switch res.Status {
		case stacks.SyncAlreadyUpToDate:
			fmt.Printf("Branch %q is already up-to-date with %q\n", currentBranch, currentTree.Branch.Parent)
		case stacks.SyncUpdated:
			fmt.Printf("Branch %q synchronized with %q\n", currentBranch, currentTree.Branch.Parent)
		case stacks.SyncConflict:
			fmt.Printf("Branch %q has merge conflict with %q, aborting...\n", currentBranch, currentTree.Branch.Parent)
			if res.Hint != "" {
				_, _ = fmt.Println(text.Indent(res.Hint, "    "))
			}
			return true, nil
		default:
			logrus.Panicf("invariant error: unknown sync result: %v", res)
		}

		if len(currentTree.Next) == 0 {
			return false, nil
		}
		if len(currentTree.Next) > 1 {
			return false, errors.Errorf("unsupported: branch %q has more than one child branch", currentBranch)
		}
		currentTree = currentTree.Next[0]
	}
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
	stackSyncCmd.Flags().StringVar(
		&stackSyncFlags.Parent, "parent", "",
		"set the stack parent to this branch",
	)
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
