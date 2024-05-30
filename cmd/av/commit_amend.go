package main

import (
	"fmt"
	"os"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/git"
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

		db, err := getDB(repo)
		if err != nil {
			return err
		}

		// We need to run git commit --amend before bubbletea grabs the terminal. Otherwise,
		// we need to make p.ReleaseTerminal() and p.RestoreTerminal().
		if err := runAmend(repo); err != nil {
			fmt.Fprint(os.Stderr, "\n", colors.Failure("Failed to amend."), "\n")
			return actions.ErrExitSilently{ExitCode: 1}
		}

		return runPostCommitRestack(repo, db)
	},
}

func runAmend(repo *git.Repo) error {
	if _, err := repo.CurrentBranchName(); err != nil {
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
		return err
	}
	return nil
}

func init() {
	commitAmendCmd.Flags().
		StringVarP(&commitAmendFlags.Message, "message", "m", "", "the commit message")
	commitAmendCmd.Flags().
		BoolVar(&commitAmendFlags.NoEdit, "no-edit", false, "amend a commit without changing its commit message")
	commitAmendCmd.MarkFlagsMutuallyExclusive("message", "no-edit")
}
