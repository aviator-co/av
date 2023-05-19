package main

import (
	"context"
	"fmt"
	"os"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/spf13/cobra"
)

var commitCreateFlags struct {
	// The commit message.
	Message string

	// Same as `git commit --all`.
	All bool
}


var commitCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "create a commit",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		currentBranchName, _ := repo.CurrentBranchName()
		currentCommitOID, err := repo.RevParse(&git.RevParse{Rev: "HEAD"})
		if err != nil {
			return errors.Errorf("cannot get the current commit object: %v", err)
		}

		if err := commitCreate(repo, currentBranchName, currentCommitOID, commitCreateFlags); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	commitCreateCmd.Flags().StringVarP(&commitCreateFlags.Message, "message", "m", "", "the commit message")
	commitCreateCmd.Flags().BoolVarP(&commitCreateFlags.All, "all", "a", false, "automatically stage modified files (same as git commit --all)")
}

 func commitCreate(repo *git.Repo, currentBranchName, currentCommitOID string, flags struct{Message string; All bool}) error {
	commitArgs := []string{"commit"}
	if commitCreateFlags.All {
		commitArgs = append(commitArgs, "--all")
	}
	if commitCreateFlags.Message != "" {
		commitArgs = append(commitArgs, "--message", commitCreateFlags.Message)
	}

	if _, err := repo.Run(&git.RunOpts{
		Args:        commitArgs,
		ExitError:   true,
		Interactive: true,
	}); err != nil {
		_, _ = fmt.Fprint(os.Stderr,
			"\n", colors.Failure("Failed to create commit."), "\n",
		)
		return errExitSilently{1}
	}

	var branchesToSync []string
	state, err := readStackSyncState(repo)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	ctx := context.Background()

	db, err := getDB(repo)
	if err != nil {
		return err
	}
	tx := db.WriteTx()
	defer tx.Abort()

	nextBranches, err := meta.SubsequentBranches(tx, currentBranchName)
	if err != nil {
		return err
	}
	branchesToSync = append(branchesToSync, nextBranches...)
	state.Branches = branchesToSync

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


	return nil
}