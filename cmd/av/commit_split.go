package main

import (
	"fmt"
	"os"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/spf13/cobra"
)

var commitSplitCmd = &cobra.Command{
	Use:          "split",
	Short:        "Split a commit into multiple commits",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
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
					"The working directory is not clean, please stash or commit them before running split command.",
				),
			)
			return errors.New("the working directory is not clean")
		}

		// Ignore errors to support a detached HEAD.
		currentBranchName := status.CurrentBranch
		currentCommitOID := status.OID
		if currentCommitOID == "" {
			fmt.Fprint(
				os.Stderr,
				colors.Failure("The repository is at the initial state."),
			)
			return errors.New("the repository is at the initial state")
		}

		// From here, we use detached HEAD, so that even if something goes wrong or user
		// aborts the operation in the middle, the original branch is intact.
		if err := splitCommit(repo, currentBranchName, currentCommitOID); err != nil {
			commitSplitAbortMessage(currentBranchName, currentCommitOID)
			return err
		}
		return nil
	},
}

func splitCommit(repo *git.Repo, currentBranchName, currentCommitOID string) error {
	if _, err := repo.Git("switch", "--detach", currentCommitOID); err != nil {
		return err
	}
	if _, err := repo.Git("reset", "--mixed", "HEAD~"); err != nil {
		return err
	}
	if _, err := repo.Git("add", "--intent-to-add", repo.Dir()); err != nil {
		return err
	}

	for {
		status, err := repo.Status()
		if err != nil {
			return errors.Errorf("cannot get the status of the repository: %v", err)
		}
		if status.IsCleanIgnoringUntracked() {
			break
		}

		if _, err := repo.Run(&git.RunOpts{
			Args:        []string{"add", "--patch"},
			ExitError:   true,
			Interactive: true,
		}); err != nil {
			return err
		}

		status, err = repo.Status()
		if err != nil {
			return errors.Errorf("cannot get the status of the repository: %v", err)
		}

		if len(status.StagedTrackedFiles) == 0 {
			return errors.New("nothing is selected to commit")
		}

		if _, err := repo.Run(&git.RunOpts{
			// Add --verbose to show the diffs to be committed.
			Args:        []string{"commit", "--verbose", "--reedit-message", currentCommitOID},
			ExitError:   true,
			Interactive: true,
		}); err != nil {
			return err
		}
	}

	if currentBranchName != "" {
		newCommitOID, err := repo.RevParse(&git.RevParse{Rev: "HEAD"})
		if err != nil {
			return errors.Errorf("cannot get the resulting commit object: %v", err)
		}
		if err := repo.UpdateRef(&git.UpdateRef{
			Ref: "refs/heads/" + currentBranchName,
			Old: currentCommitOID,
			New: newCommitOID,
			// Add this change to reflog so that the user can revert back to the
			// original state.
			CreateReflog: true,
		}); err != nil {
			return errors.Errorf(
				"cannot update the branch %s to the new commit %s: %v",
				currentBranchName,
				newCommitOID,
				err,
			)
		}
		// At this point, the HEAD is still a detached HEAD. Check out the branch.
		// repo.CheckoutBranch errors out if the repository is at the detached head.
		// We have to run git checkout in other ways.
		if _, err := repo.Git("switch", currentBranchName); err != nil {
			return errors.Errorf("cannot switch to the original branch: %v", err)
		}

		// TODO: We should rebase the stacks after split.
		_, _ = fmt.Fprint(
			os.Stderr,
			"Run 'av sync' to sync your stack if necessary.",
		)

	}

	return nil
}

func commitSplitAbortMessage(branchName, commitOID string) {
	if branchName == "" {
		// We started from a detached HEAD.
		_, _ = fmt.Fprint(
			os.Stderr,
			colors.Failure("===================================================="),
			"\n",
			colors.Failure("DETACHED HEAD"),
			"\n",
			"\n",
			"The commit split command aborted.\n",
			"The HEAD is moved to a different commit than the original commit ",
			colors.UserInput(commitOID),
			"\n",
			"\n",
			"To revert your changes and switch to the original commit ",
			colors.UserInput(commitOID),
			", run:\n",
			"\n",
			"    ",
			colors.CliCmd("git switch --discard-changes ", branchName),
			"\n",
			colors.Failure("===================================================="),
			"\n",
		)
		return
	}
	_, _ = fmt.Fprint(
		os.Stderr,
		colors.Failure("===================================================="), "\n",
		colors.Failure("DETACHED HEAD"), "\n",
		"\n",
		"The commit split command aborted.\n",
		"Your original branch ", colors.UserInput(branchName), " was not modified.\n",
		"\n",
		"Your Git repository is now in a detached HEAD state.\n",
		"To revert your changes and switch to your original (unmodified) branch, run:\n",
		"\n",
		"    ", colors.CliCmd("git switch --discard-changes ", branchName), "\n",
		colors.Failure("===================================================="), "\n",
	)
}
