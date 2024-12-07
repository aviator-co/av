package main

import (
	"fmt"
	"os"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/colors"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
)

var stackSplitCmd = &cobra.Command{
	Use:          "split",
	Short:        "Split the last commit of the current branch into a new branch",
	SilenceUsage: true,
	Args:         cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		newBranchName := ""

		if len(args) > 0 {
			// Use the provided argument as the new branch name
			newBranchName = args[0]
		}
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

	// Generate a branch name if none is provided
	if newBranchName == "" {
		sanitizedMessage := sanitizeBranchName(lastCommit.Message)
		newBranchName = fmt.Sprintf("%s", sanitizedMessage)
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

	// Switch to the newly created branch
	worktree, err := repo.GoGitRepo().Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	if err := worktree.Checkout(&gogit.CheckoutOptions{
		Branch: newBranchRefName,
		Create: false, // We're switching, not creating
		Force:  false, // Avoid overwriting changes
	}); err != nil {
		return fmt.Errorf("failed to switch to new branch '%s': %w", newBranchName, err)
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

// sanitizeBranchName creates a valid branch name from a string
func sanitizeBranchName(input string) string {
	sanitized := strings.ToLower(strings.TrimSpace(input))
	sanitized = strings.ReplaceAll(sanitized, " ", "-")
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "\\", "-")
	sanitized = strings.ReplaceAll(sanitized, ".", "-")
	sanitized = strings.ReplaceAll(sanitized, ":", "-")
	// Limit length for branch names
	if len(sanitized) > 40 {
		sanitized = sanitized[:40]
	}
	return sanitized
}
