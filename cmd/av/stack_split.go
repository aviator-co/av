package main

import (
	"fmt"
	"os"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
)

var stackSplitCmd = &cobra.Command{
	Use:          "split",
	Short:        "Split the last commit of the current branch into a new branch",
	SilenceUsage: true,
	Args:         cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		newBranchName := args[0]
		repo, err := getRepo()
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
					"The working directory is not clean. Please stash or commit changes before running the split command.",
				),
			)
			return errors.New("the working directory is not clean")
		}

		currentBranchName := status.CurrentBranch
		currentCommitOID := status.OID
		if currentCommitOID == "" {
			fmt.Fprint(
				os.Stderr,
				colors.Failure("The repository is in an initial state with no commits."),
			)
			return errors.New("the repository has no commits")
		}

		if err := splitLastCommit(repo, currentBranchName, newBranchName); err != nil {
			splitAbortMessage(currentBranchName)
			return err
		}
		return nil
	},
}

func splitLastCommit(repo *git.Repo, currentBranchName string, newBranchName string) error {
	// Create a new branch from the current HEAD
	if newBranchName == "" {
		return errors.New("new branch name must be provided")
	}

	// Get the current branch reference
	currentBranchRefName := plumbing.NewBranchReferenceName(currentBranchName)
	currentBranchRef, err := repo.GoGitRepo().Reference(currentBranchRefName, true)
	if err != nil {
		return fmt.Errorf("failed to get reference for current branch %s: %w", currentBranchName, err)
	}

	lastCommitHash := currentBranchRef.Hash()
	// Get the last commit object
	lastCommit, err := repo.GoGitRepo().CommitObject(lastCommitHash)
	if err != nil {
		return fmt.Errorf("failed to get last commit: %w", err)
	}
	// Get the parent of the last commit (HEAD~1)
	parentCommitIter := lastCommit.Parents()
	parentCommit, err := parentCommitIter.Next()
	if err != nil {
		return fmt.Errorf("failed to get parent commit: %w", err)
	}

	// Create a new branch pointing to the last commit
	newBranchRefName := plumbing.NewBranchReferenceName(newBranchName)
	newBranchRef := plumbing.NewHashReference(newBranchRefName, lastCommitHash)
	if err := repo.GoGitRepo().Storer.SetReference(newBranchRef); err != nil {
		return fmt.Errorf("failed to create new branch: %w", err)
	}

	updatedCurrentBranchRef := plumbing.NewHashReference(currentBranchRefName, parentCommit.Hash)
	if err := repo.GoGitRepo().Storer.SetReference(updatedCurrentBranchRef); err != nil {
		return fmt.Errorf("failed to update current branch: %w", err)
	}

	fmt.Fprint(
		os.Stdout,
		colors.Success(fmt.Sprintf("Successfully split the last commit into a new branch %s.\n", newBranchName)),
	)

	return nil
}

func splitAbortMessage(branchName string) {
	_, _ = fmt.Fprint(
		os.Stderr,
		colors.Failure("====================================================\n"),
		"The split operation was aborted.\n",
		"The original branch ", colors.UserInput(branchName), " remains intact.\n",
		"Your Git repository is now in a detached HEAD state.\n",
		"To revert to your original branch and commit, run:\n",
		"    ", colors.CliCmd(fmt.Sprintf("git switch %s", branchName)), "\n",
		colors.Failure("====================================================\n"),
	)
}
