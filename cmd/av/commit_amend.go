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

var commitAmendFlags struct {
	// The commit message to update with.
	Message string

	// Same as `git commit --amend --no-edit`. Amends a commit without changing its commit message.
	NoEdit bool
}


var commitAmendCmd = &cobra.Command{
	Use:   "amend",
	Short: "amend a commit",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		currentBranchName, err := repo.CurrentBranchName()
		if err != nil {
			return errors.WrapIf(err, "failed to determine current branch")
		}

		commitArgs := []string{"commit", "--amend"}
		if commitAmendFlags.NoEdit {
			commitArgs = append(commitArgs, "--no-edit")
		}
		if commitAmendFlags.Message != "" {
			commitArgs = append(commitArgs, "--message", commitAmendFlags.Message)
		}

		if _, err := repo.Run(&git.RunOpts{
			Args:        commitArgs,
			ExitError:   true,
			Interactive: true,
		}); err != nil {
			_, _ = fmt.Fprint(os.Stderr,
				"\n", colors.Failure("Failed to amend."), "\n",
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

		err = actions.SyncStack(ctx, repo, client, tx, branchesToSync, state, syncFlags)
		if err != nil {
			return err
		}

		return nil
	},
}

func init() {
	commitAmendCmd.Flags().StringVarP(&commitAmendFlags.Message, "message", "m", "", "the commit message")
	commitAmendCmd.Flags().BoolVar(&commitAmendFlags.NoEdit, "no-edit", false, "amend a commit without changing its commit message")
	commitAmendCmd.MarkFlagsMutuallyExclusive("message", "no-edit")
}