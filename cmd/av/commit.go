package main

import (
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

var commitFlags struct {
	Message string
	All     bool
	Amend   bool
	Edit    bool
}

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Record changes to the repository with commits",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if commitFlags.Amend {
			return amendCmd(commitFlags.Message, commitFlags.Edit, commitFlags.All)
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

func init() {
	commitCmd.Flags().
		StringVarP(&commitFlags.Message, "message", "m", "", "the commit message")
	commitCmd.Flags().
		BoolVarP(&commitFlags.All, "all", "a", false, "automatically stage modified files (same as git commit --all)")
	commitCmd.Flags().
		BoolVarP(&commitFlags.Amend, "amend", "", false, "amend the last commit")
	commitCmd.Flags().
		BoolVarP(&commitFlags.Edit, "edit", "", false, "edit an amended commit's message")

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
