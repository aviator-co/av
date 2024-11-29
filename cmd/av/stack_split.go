package main

import (
	"fmt"
	"os"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/colors"
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

	if _, err := repo.Git("switch", "-c", newBranchName); err != nil {
		return errors.Errorf("failed to create a new branch %s: %v", newBranchName, err)
	}

	// Reset the current branch to HEAD~1
	if _, err := repo.Git("switch", currentBranchName); err != nil {
		return errors.Errorf("failed to switch to the current branch %s: %v", currentBranchName, err)
	}
	if _, err := repo.Git("reset", "--hard", "HEAD~1"); err != nil {
		return errors.Errorf("failed to reset the branch to the previous commit: %v", err)
	}

	// Show all branches after successful split
	branchesOutput, err := repo.Git("branch", "--list")
	if err != nil {
		return errors.Errorf("failed to list branches: %v", err)
	}

	fmt.Fprint(
		os.Stdout,
		colors.Success(fmt.Sprintf("Successfully split the last commit into a new branch %s.\n", newBranchName)),
	)
	fmt.Fprint(
		os.Stdout,
		colors.Success("Current branches in the repository:\n"),
		branchesOutput,
		"\n",
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
