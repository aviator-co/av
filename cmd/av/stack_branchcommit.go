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

var stackBranchCommitFlags struct {
	// The commit message.
	Message string

	// Name of the new branch.
	BranchName string

	// Same as `git add --all`.
	// Stages all changes, including untracked files.
	All bool

	// Same as `git commit --all`.
	// Stage all files that have been modified and deleted, but ignore untracked files.
	AllModified bool
}

var stackBranchCommitCmd = &cobra.Command{
	Use:          "branch-commit [flags]",
	Aliases:      []string{"branchcommit", "bc"},
	Short:        "create a new stacked branch and commit staged changes to it",
	Long:         "Create a new branch that is stacked on the current branch and commit all staged changes with the specified arguments.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) (reterr error) {
		branchName := stackBranchCommitFlags.BranchName
		if branchName == "" {
			if stackBranchCommitFlags.Message == "" {
				_ = cmd.Usage()
				return errors.New("Need a branch name or a commit message")
			}
			branchName = branchNameFromMessage(stackBranchCommitFlags.Message)
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

		// Determine important contextual information from Git
		defaultBranch, err := repo.DefaultBranch()
		if err != nil {
			return errors.WrapIf(err, "failed to determine repository default branch")
		}

		parentBranchName, err := repo.CurrentBranchName()
		if err != nil {
			return errors.WrapIff(err, "failed to get current branch name")
		}

		// Currently, we only allow the repo default branch to be a trunk.
		// We might want to allow other branches to be trunks in the future, but
		// that does run the risk of allowing the user to get into a weird state
		// (where some stacks assume a branch is a trunk and others don't).
		isBranchFromTrunk := parentBranchName == defaultBranch
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
			_, _ = fmt.Fprint(os.Stderr,
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
		if stackBranchCommitFlags.All {
			addArgs = append(addArgs, "--all")
		} else if stackBranchCommitFlags.AllModified {
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
				_, _ = fmt.Fprint(os.Stderr,
					"\n", colors.Failure("Failed to stage files: ", err.Error()), "\n",
				)
				return actions.ErrExitSilently{ExitCode: 1}
			}
		}

		commitArgs := []string{"commit"}
		if stackBranchCommitFlags.Message != "" {
			commitArgs = append(commitArgs, "--message", stackBranchCommitFlags.Message)
		}

		if _, err := repo.Run(&git.RunOpts{
			Args:        commitArgs,
			ExitError:   true,
			Interactive: true,
		}); err != nil {
			_, _ = fmt.Fprint(os.Stderr,
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
	},
}

func init() {
	stackBranchCommitCmd.Flags().
		StringVarP(&stackBranchCommitFlags.Message, "message", "m", "", "the commit message")
	stackBranchCommitCmd.Flags().
		StringVarP(&stackBranchCommitFlags.BranchName, "branch-name", "b", "", "the branch name to create (if empty, automatically generated from the message)")
	stackBranchCommitCmd.Flags().
		BoolVarP(&stackBranchCommitFlags.All, "all", "A", false, "automatically stage all files")
	stackBranchCommitCmd.Flags().
		BoolVarP(&stackBranchCommitFlags.AllModified, "all-modified", "a", false, "automatically stage modified and deleted files (ignore untracked files)")

	stackBranchCommitCmd.MarkFlagsMutuallyExclusive("all", "all-modified")
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
