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

var squashCmd = &cobra.Command{
	Use:   "squash",
	Short: "Squash commits of the current branch into a single commit",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		repo, err := getRepo(ctx)
		if err != nil {
			return err
		}

		db, err := getDB(ctx, repo)
		if err != nil {
			return err
		}

		if err := runSquash(ctx, repo, db); err != nil {
			fmt.Fprint(os.Stderr, colors.Failure("Failed to squash."), "\n")
			fmt.Fprint(os.Stderr, colors.Failure(err.Error()), "\n")
			return actions.ErrExitSilently{ExitCode: 1}
		}

		return runPostCommitRestack(repo, db)
	},
}

func runSquash(ctx context.Context, repo *git.Repo, db meta.DB) error {
	status, err := repo.Status(ctx)
	if err != nil {
		return errors.Errorf("cannot get the status of the repository: %v", err)
	}

	if !status.IsClean() {
		return errors.New(
			"the working directory is not clean, please stash or commit them before running squash command.",
		)
	}

	currentBranchName, err := repo.CurrentBranchName(ctx)
	if err != nil {
		return err
	}

	tx := db.WriteTx()
	defer tx.Abort()

	branch, branchExists := tx.Branch(currentBranchName)
	if !branchExists {
		return errors.New("current branch does not exist in the database")
	}

	if branch.PullRequest != nil &&
		branch.PullRequest.State == githubv4.PullRequestStateMerged {
		return errors.New("this branch has already been merged, squashing is not allowed")
	}

	commitIDs, err := repo.RevList(ctx, git.RevListOpts{
		Specifiers: []string{currentBranchName, "^" + branch.Parent.Name},
		Reverse:    true,
	})
	if err != nil {
		return err
	}

	if len(commitIDs) <= 1 {
		return errors.New("no commits to squash")
	}

	firstCommitSha := commitIDs[0]

	if _, err := repo.Git(ctx, "reset", "--soft", firstCommitSha); err != nil {
		return err
	}

	amendMessage, err := repo.Git(ctx, "commit", "--amend", "--no-edit")
	if err != nil {
		return err
	}

	fmt.Fprint(
		os.Stderr,
		"\n",
		colors.Success(fmt.Sprintf("Successfully squashed %d commits", len(commitIDs))),
		"\n",
	)
	fmt.Fprint(os.Stderr, amendMessage, "\n\n")
	return nil
}
