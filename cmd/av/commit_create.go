package main

import (
	"context"
	"fmt"
	"os"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/shurcooL/githubv4"
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

		db, err := getDB(repo)
		if err != nil {
			return err
		}

		// We need to run git commit before bubbletea grabs the terminal. Otherwise,
		// we need to make p.ReleaseTerminal() and p.RestoreTerminal().
		if err := runCreate(repo, db); err != nil {
			fmt.Fprint(os.Stderr, "\n", colors.Failure("Failed to create commit."), "\n")
			return actions.ErrExitSilently{ExitCode: 1}
		}

		return runPostCommitRestack(repo, db)
	},
}

func runCreate(repo *git.Repo, db meta.DB) error {
	currentBranch, err := repo.CurrentBranchName()
	if err != nil {
		return errors.WrapIf(err, "failed to determine current branch")
	}

	commitArgs := []string{"commit"}
	if commitCreateFlags.All {
		commitArgs = append(commitArgs, "--all")
	}
	if commitCreateFlags.Message != "" {
		commitArgs = append(commitArgs, "--message", commitCreateFlags.Message)
	}

	writeTx := db.WriteTx()
	defer writeTx.Abort()

	client, err := getGitHubClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	prUpdateResult, err := actions.UpdatePullRequestState(ctx, client, writeTx, currentBranch)
	if err != nil {
		return err
	}

	if prUpdateResult.Pull != nil && prUpdateResult.Pull.State == githubv4.PullRequestStateMerged {
		return errors.New("this branch has already been merged, commit is not allowed")
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
	commitCreateCmd.Flags().
		StringVarP(&commitCreateFlags.Message, "message", "m", "", "the commit message")
	commitCreateCmd.Flags().
		BoolVarP(&commitCreateFlags.All, "all", "a", false, "automatically stage modified files (same as git commit --all)")
}
