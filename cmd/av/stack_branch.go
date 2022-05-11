package main

import (
	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/cleanup"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var stackBranchFlags struct {
	// The parent branch to base the new branch off.
	// By default, this is the current branch.
	Parent string
}
var stackBranchCmd = &cobra.Command{
	Use:   "branch [flags] <branch-name>",
	Short: "create a new stacked branch",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			_ = cmd.Usage()
			return errors.New("exactly one branch name is required")
		}
		name := args[0]

		repo, err := getRepo()
		if err != nil {
			return err
		}

		// Validate preconditions
		if _, err := repo.RevParse(&git.RevParse{Rev: name}); err == nil {
			return errors.Errorf("branch %q already exists", name)
		}

		// Determine important contextual information from Git
		defaultBranch, err := repo.DefaultBranch()
		if err != nil {
			return errors.WrapIf(err, "failed to determine repository default branch")
		}

		// Determine the parent branch and make sure it's checked out
		var parentBranch string
		var cu cleanup.Cleanup
		defer cu.Cleanup()
		if stackBranchFlags.Parent != "" {
			parentBranch = stackBranchFlags.Parent
			origBranch, err := repo.CheckoutBranch(&git.CheckoutBranch{Name: parentBranch})
			if err != nil {
				return errors.WrapIf(err, "failed to checkout parent branch")
			}
			cu.Add(func() {
				if _, err := repo.CheckoutBranch(&git.CheckoutBranch{Name: origBranch}); err != nil {
					logrus.WithError(err).Warn("cleanup error: failed to return to original branch")
				}
			})
		} else {
			var err error
			parentBranch, err = repo.CurrentBranchName()
			if err != nil {
				return errors.WrapIff(err, "failed to get current branch name")
			}
		}

		// Special case: branching from the repository default branch
		// We set parentBranch = "" as a sentinel value to indicate that this
		// branch is the root of a new stack.
		if parentBranch == defaultBranch {
			logrus.Debug("creating new stack root branch from default branch")
			parentBranch = ""
		}

		// Create a new branch off of the parent
		if _, err := repo.CheckoutBranch(&git.CheckoutBranch{
			Name:      name,
			NewBranch: true,
		}); err != nil {
			logrus.WithError(err).Debugf("failed to checkout branch %q", name)
			return errors.Errorf(
				"failed to create branch %q (does it already exist?)",
				name,
			)
		}

		branchMeta := meta.Branch{Name: name, Parent: parentBranch}
		logrus.WithField("meta", branchMeta).Debug("writing branch metadata")
		if err := meta.WriteBranch(repo, branchMeta); err != nil {
			return errors.WrapIff(err, "failed to write av internal metadata for branch %q", name)
		}

		// If this isn't a new stack root, update the parent metadata to include
		// the new branch as a child.
		if parentBranch != "" {
			parentMeta, _ := meta.ReadBranch(repo, parentBranch)
			parentMeta.Children = append(parentMeta.Children, name)
			logrus.WithField("meta", parentMeta).Debug("writing parent branch metadata")
			if err := meta.WriteBranch(repo, parentMeta); err != nil {
				return errors.WrapIf(err, "failed to write parent branch metadata")
			}
		}

		cu.Cancel()
		return nil
	},
}
