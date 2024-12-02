package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/cleanup"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	// This is an arbitrary limit on the branch name length.
	branchNameLength = 200
)

var (
	// See man 1 git-check-ref-format for the refname spec. This pattern is more restrictive
	// than the spec.
	//
	// * Do not allow slashes because creating a branch directory from a commit message is
	//   unlikely.
	// * Do not allow dots because dots cannot be placed on a certain location and it's unlikely
	//   the user wants to have a dot in the branch name.
	branchNameReplacedPattern = regexp.MustCompile("[^-_a-zA-Z0-9]")

	multipleSpacePattern = regexp.MustCompile(" +")
)

var commitFlags struct {
	Message    string
	All        bool
	Amend      bool
	Edit       bool
	BranchName string
	AllChanges bool
}

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Record changes to the repository with commits",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if commitFlags.Amend {
			return amendCmd(commitFlags.Message, commitFlags.Edit, commitFlags.All)
		}

		if commitFlags.BranchName != "" {
			return branchAndCommit(
				commitFlags.BranchName,
				commitFlags.Message,
				commitFlags.AllChanges,
				commitFlags.All,
			)

		}

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
	if commitFlags.All {
		commitArgs = append(commitArgs, "--all")
	}
	if commitFlags.Message != "" {
		commitArgs = append(commitArgs, "--message", commitFlags.Message)
	}

	tx := db.WriteTx()
	defer tx.Abort()

	branch, _ := tx.Branch(currentBranch)
	if branch.PullRequest != nil && branch.PullRequest.State == githubv4.PullRequestStateMerged {
		fmt.Fprint(
			os.Stderr,
			colors.Failure("This branch has already been merged, commit is not allowed"),
			"\n",
		)
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

func amendCmd(message string, edit bool, all bool) error {
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
	if err := runAmend(repo, db, message, edit, all); err != nil {
		fmt.Fprint(os.Stderr, "\n", colors.Failure("Failed to amend."), "\n")
		return actions.ErrExitSilently{ExitCode: 1}
	}

	return runPostCommitRestack(repo, db)
}

func runAmend(repo *git.Repo, db meta.DB, message string, edit bool, all bool) error {
	currentBranch, err := repo.CurrentBranchName()
	if err != nil {
		return errors.WrapIf(err, "failed to determine current branch")
	}

	commitArgs := []string{"commit", "--amend"}
	if !edit && message == "" {
		commitArgs = append(commitArgs, "--no-edit")
	}
	if all {
		commitArgs = append(commitArgs, "--all")
	}
	if message != "" {
		commitArgs = append(commitArgs, "--message", message)
	}

	tx := db.WriteTx()
	defer tx.Abort()

	branch, _ := tx.Branch(currentBranch)
	if branch.PullRequest != nil && branch.PullRequest.State == githubv4.PullRequestStateMerged {
		fmt.Fprint(
			os.Stderr,
			colors.Failure("This branch has already been merged, amending is not allowed"),
			"\n",
		)
		return errors.New("this branch has already been merged, amending is not allowed")
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

func branchAndCommit(branchName string, message string, all bool, allModified bool) (reterr error) {
	if branchName == "" || branchName == "AV_CLI_TEMP_BRANCH" {
		if message == "" {
			return errors.New("Need a branch name or a commit message")
		}
		branchName = branchNameFromMessage(message)
		if branchName == "" {
			return errors.New("Cannot create a valid branch name from the message")
		}
	}

	repo, err := getRepo()
	if err != nil {
		return err
	}

	db, err := getDB(repo)
	if err != nil {
		return err
	}
	tx := db.WriteTx()
	var cu cleanup.Cleanup
	defer cu.Cleanup()
	cu.Add(func() {
		logrus.WithError(reterr).Debug("aborting db transaction")
		tx.Abort()
	})

	parentBranchName, err := repo.CurrentBranchName()
	if err != nil {
		return errors.WrapIff(err, "failed to get current branch name")
	}

	// Currently, we only allow the repo default branch to be a trunk.
	// We might want to allow other branches to be trunks in the future, but
	// that does run the risk of allowing the user to get into a weird state
	// (where some stacks assume a branch is a trunk and others don't).
	isBranchFromTrunk, err := repo.IsTrunkBranch(parentBranchName)
	if err != nil {
		return errors.WrapIf(err, "failed to check if branch is trunk")
	}
	var parentHead string
	if !isBranchFromTrunk {
		parentHead, err = repo.RevParse(&git.RevParse{Rev: parentBranchName})
		if err != nil {
			return errors.WrapIf(err, "failed to get parent branch head commit")
		}
	}

	if err != nil {
		return errors.WrapIf(err, "failed to read parent branch state")
	}

	// Create a new branch off of the parent
	logrus.WithFields(logrus.Fields{
		"parent":     parentBranchName,
		"new_branch": branchName,
	}).Debug("creating new branch from parent")
	if _, err := repo.CheckoutBranch(&git.CheckoutBranch{
		Name:      branchName,
		NewBranch: true,
	}); err != nil {
		return errors.WrapIff(err, "checkout error")
	}

	// On failure, we want to delete the branch we created so that the user
	// can try again (e.g., to fix issues surfaced by a pre-commit hook).
	cu.Add(func() {
		fmt.Fprint(os.Stderr,
			colors.Faint("  - Cleaning up branch "),
			colors.UserInput(branchName),
			colors.Faint(" because commit was not successful."),
			"\n",
		)
		if _, err := repo.CheckoutBranch(&git.CheckoutBranch{
			Name: parentBranchName,
		}); err != nil {
			logrus.WithError(err).Error("failed to return to original branch during cleanup")
		}
		if err := repo.BranchDelete(branchName); err != nil {
			logrus.WithError(err).Error("failed to delete branch during cleanup")
		}
	})

	tx.SetBranch(meta.Branch{
		Name: branchName,
		Parent: meta.BranchState{
			Name:  parentBranchName,
			Trunk: isBranchFromTrunk,
			Head:  parentHead,
		},
	})

	// For "--all" and "--all-modified",
	var addArgs []string
	if all {
		addArgs = append(addArgs, "--all")
	} else if allModified {
		// This is meant to mirror `git commit --all` which does not add
		// unstaged files to the index (which is different from `git add --all`).
		addArgs = append(addArgs, "--update")
	}
	if len(addArgs) > 0 {
		_, err := repo.Run(&git.RunOpts{
			Args:      append([]string{"add"}, addArgs...),
			ExitError: true,
		})
		if err != nil {
			fmt.Fprint(os.Stderr,
				"\n", colors.Failure("Failed to stage files: ", err.Error()), "\n",
			)
			return actions.ErrExitSilently{ExitCode: 1}
		}
	}

	commitArgs := []string{"commit"}
	if message != "" {
		commitArgs = append(commitArgs, "--message", message)
	}

	if _, err := repo.Run(&git.RunOpts{
		Args:        commitArgs,
		ExitError:   true,
		Interactive: true,
	}); err != nil {
		fmt.Fprint(os.Stderr,
			"\n", colors.Failure("Failed to create commit."), "\n",
		)
		return actions.ErrExitSilently{ExitCode: 1}
	}

	// Cancel the cleanup **after** the commit is successful (so that we
	// delete the created branch).
	cu.Cancel()
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func branchNameFromMessage(message string) string {
	name := branchNameReplacedPattern.ReplaceAllLiteralString(message, " ")
	name = strings.TrimSpace(name)
	name = multipleSpacePattern.ReplaceAllLiteralString(name, "-")
	if len(name) > branchNameLength {
		name = name[:branchNameLength]
	}
	name = strings.ToLower(name)
	if config.Av.PullRequest.BranchNamePrefix != "" {
		name = fmt.Sprintf("%s%s", config.Av.PullRequest.BranchNamePrefix, name)
	}
	return name
}

func init() {
	commitCmd.Flags().
		StringVarP(&commitFlags.Message, "message", "m", "", "the commit message")
	commitCmd.Flags().
		BoolVarP(&commitFlags.All, "all", "a", false, "automatically stage modified files (same as git commit --all)")
	commitCmd.Flags().
		BoolVarP(&commitFlags.AllChanges, "all-changes", "A", false, "all files, including untracked and deleted files")
	commitCmd.Flags().
		BoolVar(&commitFlags.Amend, "amend", false, "amend the last commit")
	commitCmd.Flags().
		BoolVar(&commitFlags.Edit, "edit", false, "edit an amended commit's message")
	commitCmd.Flags().
		StringVarP(&commitFlags.BranchName, "branch-name", "b", "",
			"the branch name to create (if empty, automatically generated from the message)")
	commitCmd.Flags().Lookup("branch-name").NoOptDefVal = "AV_CLI_TEMP_BRANCH"

	commitCmd.MarkFlagsMutuallyExclusive("amend", "branch-name")

	deprecatedAmendCmd := deprecateCommand(*commitAmendCmd, "av commit --amend", "amend")
	deprecatedAmendCmd.Hidden = true
	deprecatedCreateCmd := deprecateCommand(*commitCmd, "av commit", "create")
	deprecatedCreateCmd.Hidden = true
	deprecatedSplitCmd := deprecateCommand(*splitCommitCmd, "av split-commit", "split")
	deprecatedSplitCmd.Hidden = true

	deprecatedAmendCmd.Flags().
		StringVarP(&commitAmendFlags.Message, "message", "m", "", "the commit message")
	deprecatedAmendCmd.Flags().
		BoolVar(&commitAmendFlags.NoEdit, "no-edit", false, "amend a commit without changing its commit message")
	deprecatedAmendCmd.MarkFlagsMutuallyExclusive("message", "no-edit")
	deprecatedAmendCmd.Flags().
		BoolVarP(&commitAmendFlags.All, "all", "a", false, "automatically stage modified files (same as git commit --all)")

	commitCmd.AddCommand(
		deprecatedAmendCmd,
		deprecatedCreateCmd,
		deprecatedSplitCmd,
	)

}
