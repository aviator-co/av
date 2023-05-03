package main

import (
	"github.com/aviator-co/av/internal/utils/cleanup"
	"regexp"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
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

	// Same as `git commit --all`.
	All bool
}

var stackBranchCommitCmd = &cobra.Command{
	Use:          "branch-commit [flags]",
	Short:        "create a new stacked branch and a commit",
	Long:         "Create a new branch that is stacked on the current branch, and call git-commit with the specified arguments.",
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

		tx.SetBranch(meta.Branch{
			Name: branchName,
			Parent: meta.BranchState{
				Name:  parentBranchName,
				Trunk: isBranchFromTrunk,
				Head:  parentHead,
			},
		})

		// If this isn't a new stack root, update the parent metadata to include
		// the new branch as a child.
		if !isBranchFromTrunk {
			parentMeta, _ := tx.Branch(parentBranchName)
			parentMeta.Children = append(parentMeta.Children, branchName)
			logrus.WithField("meta", parentMeta).Debug("writing parent branch metadata")
			tx.SetBranch(parentMeta)
		}

		// TODO[UX]
		// Here, we commit the db changes before actually creating a git commit
		// since the git commit might fail. It's a little unclear what should
		// actually happen if that occurs -- at this point we've already checked
		// out the branch. Should we delete the branch? That would allow the
		// user to fix any issues (e.g., those surfaced by pre-commit hooks) and
		// re-run the `av stack branch-commit` command verbatim again.
		cu.Cancel()
		if err := tx.Commit(); err != nil {
			return err
		}

		commitArgs := []string{"commit"}
		if stackBranchCommitFlags.All {
			commitArgs = append(commitArgs, "--all")
		}
		if stackBranchCommitFlags.Message != "" {
			commitArgs = append(commitArgs, "--message", stackBranchCommitFlags.Message)
		}

		if _, err := repo.Run(&git.RunOpts{
			Args:        commitArgs,
			ExitError:   true,
			Interactive: true,
		}); err != nil {
			return errors.WrapIff(err, "failed to create a commit")
		}

		return nil
	},
}

func init() {
	stackBranchCommitCmd.Flags().StringVarP(&stackBranchCommitFlags.Message, "message", "m", "", "commit message")
	stackBranchCommitCmd.Flags().StringVarP(&stackBranchCommitFlags.BranchName, "branch-name", "b", "", "branch name. If empty, auto-generated from the commit message")
	stackBranchCommitCmd.Flags().BoolVarP(&stackBranchCommitFlags.All, "all", "a", false, "same as git commit --all")
}

func branchNameFromMessage(message string) string {
	name := branchNameReplacedPattern.ReplaceAllLiteralString(message, " ")
	name = strings.TrimSpace(name)
	name = multipleSpacePattern.ReplaceAllLiteralString(name, "-")
	if len(name) > branchNameLength {
		name = name[:branchNameLength]
	}
	return name
}
