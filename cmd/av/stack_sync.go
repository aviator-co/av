package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/stringutils"
	"github.com/kr/text"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

var stackSyncFlags struct {
	actions.StackSyncConfig

	All      bool
	Abort    bool
	Continue bool
	Skip     bool
}

var stackSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize stacked branches",
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
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		repo, err := getRepo()
		if err != nil {
			return err
		}
		db, err := getDB(repo)
		if err != nil {
			return err
		}
		if _, err = actions.TidyDB(repo, db); err != nil {
			return err
		}

		tx := db.WriteTx()
		defer tx.Abort()

		// Read any preexisting state.
		// This is required to allow us to handle --continue/--abort/--skip
		state, err := actions.ReadStackSyncState(repo)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		if stackSyncFlags.Abort {
			if state.CurrentBranch == "" || state.Continuation == nil {
				// Try to clear the state file if it exists just to be safe.
				_ = actions.WriteStackSyncState(repo, nil)
				return errors.New("no sync in progress")
			}

			// Abort the rebase if we need to
			if stat, _ := os.Stat(filepath.Join(repo.GitDir(), "REBASE_HEAD")); stat != nil {
				if _, err := repo.Rebase(git.RebaseOpts{Abort: true}); err != nil {
					return errors.WrapIf(err, "failed to abort in-progress rebase")
				}
			}

			err := actions.WriteStackSyncState(repo, nil)
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
				return errors.New(
					"refusing to sync: there are unstaged changes in the working tree (use `git add` to stage changes)",
				)
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
			state.Config = actions.StackSyncConfig{
				Current: stackSyncFlags.Current,
				Trunk:   stackSyncFlags.Trunk,
				NoPush:  stackSyncFlags.NoPush,
				NoFetch: stackSyncFlags.NoFetch,
				Parent:  stackSyncFlags.Parent,
				Prune:   stackSyncFlags.Prune,
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
				if err := actions.WriteStackSyncState(repo, &state); err != nil {
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
		} else if stackSyncFlags.All {
			for _, br := range tx.AllBranches() {
				if !br.IsStackRoot() {
					continue
				}
				branchesToSync = append(branchesToSync, br.Name)
				nextBranches := meta.SubsequentBranches(tx, branchesToSync[len(branchesToSync)-1])
				branchesToSync = append(branchesToSync, nextBranches...)
			}
			state.Branches = branchesToSync
		} else {
			currentBranch, err := repo.CurrentBranchName()
			if err != nil {
				return err
			}
			branchesToSync, err = meta.StackBranches(tx, currentBranch)
			if err != nil {
				return err
			}
			state.Branches = branchesToSync
		}
		// Either way (--continue or not), we sync all subsequent branches

		logrus.WithField("branches", branchesToSync).Debug("determined branches to sync")
		client, err := getGitHubClient()
		if err != nil {
			return err
		}

		var syncOpts []actions.SyncStackOpt
		if stackSyncFlags.Skip {
			syncOpts = append(syncOpts, actions.WithSkipNextCommit())
		}
		err = actions.SyncStack(ctx, repo, client, tx, branchesToSync, state, syncOpts...)
		if err != nil {
			return err
		}

		return nil
	},
}

func init() {
	stackSyncCmd.Flags().BoolVar(
		&stackSyncFlags.All, "all", false,
		"synchronize all branches",
	)
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
	stackSyncCmd.Flags().BoolVar(
		&stackSyncFlags.Prune, "prune", false,
		"delete the merged branches",
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

	stackSyncCmd.MarkFlagsMutuallyExclusive("current", "all")
	stackSyncCmd.MarkFlagsMutuallyExclusive("current", "trunk")
	stackSyncCmd.MarkFlagsMutuallyExclusive("trunk", "parent")
	stackSyncCmd.MarkFlagsMutuallyExclusive("continue", "abort", "skip")
}
