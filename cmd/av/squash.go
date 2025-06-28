package main

import (
	"fmt"
	"os"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/shurcooL/githubv4"
	"github.com/spf13/cobra"
)

var squashCmd = &cobra.Command{
	Use:   "squash",
	Short: "Squash commits of the current branch into a single commit",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		db, err := getDB(repo)
		if err != nil {
			return err
		}

		status, err := repo.Status()
		if err != nil {
			return errors.Errorf("cannot get the status of the repository: %v", err)
		}
		if !status.IsClean() {
			fmt.Fprint(
				os.Stderr,
				colors.Failure(
					"The working directory is not clean, please stash or commit them before running squash command.",
				),
			)
			return errors.New("the working directory is not clean")
		}

		currentBranchName, err := repo.CurrentBranchName()
		if err != nil {
			return err
		}

		branch, branchExists := db.WriteTx().Branch(currentBranchName)
		if !branchExists {
			return errors.New("current branch does not exist in the database")
		}

		if branch.PullRequest != nil && branch.PullRequest.State == githubv4.PullRequestStateMerged {
			fmt.Fprint(
				os.Stderr,
				colors.Failure("This branch has already been merged, squashing is not allowed"),
				"\n",
			)
			return errors.New("this branch has already been merged, squashing is not allowed")
		}

		commitIDs, err := repo.RevList(git.RevListOpts{
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

		if _, err := repo.Git("reset", "--soft", firstCommitSha); err != nil {
			return err
		}

		if _, err := repo.Git("commit", "--amend", "--no-edit"); err != nil {
			return err
		}

		return runPostCommitRestack(repo, db)
	},
}
