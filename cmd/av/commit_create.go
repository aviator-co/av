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

		currentBranchName, err := repo.CurrentBranchName()
		if err != nil {
			return errors.WrapIf(err, "failed to determine current branch")
		}

		if err := commitCreate(repo, currentBranchName, commitCreateFlags); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	commitCreateCmd.Flags().
		StringVarP(&commitCreateFlags.Message, "message", "m", "", "the commit message")
	commitCreateCmd.Flags().
		BoolVarP(&commitCreateFlags.All, "all", "a", false, "automatically stage modified files (same as git commit --all)")
}

func commitCreate(repo *git.Repo, currentBranchName string, flags struct {
	Message string
	All     bool
}) error {
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
		return actions.ErrExitSilently{ExitCode: 1}
	}

	state, err := actions.ReadStackSyncState(repo)
	state.OriginalBranch = currentBranchName

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

	client, err := getClient(config.Av.GitHub.Token)
	if err != nil {
		return err
	}

	branchesToSync, err := meta.SubsequentBranches(tx, currentBranchName)
	if err != nil {
		return err
	}

	err = actions.SyncStack(ctx, repo, client, tx, branchesToSync, state)
	if err != nil {
		return err
	}

	return nil
}
