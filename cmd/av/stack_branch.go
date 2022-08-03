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

		// Determine important contextual information from Git
		defaultBranch, err := repo.DefaultBranch()
		if err != nil {
			return errors.WrapIf(err, "failed to determine repository default branch")
		}

		// Determine the parent branch and make sure it's checked out
		var parentBranchName string
		var cu cleanup.Cleanup
		defer cu.Cleanup()
		if stackBranchFlags.Parent != "" {
			parentBranchName = stackBranchFlags.Parent
			origBranch, err := repo.CheckoutBranch(&git.CheckoutBranch{Name: parentBranchName})
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
			parentBranchName, err = repo.CurrentBranchName()
			if err != nil {
				return errors.WrapIff(err, "failed to get current branch name")
			}
		}

		// Currently, we only allow the repo default branch to be a trunk.
		// We might want to allow other branches to be trunks in the future, but
		// that does run the risk of allowing the user to get into a weird state
		// (where some stacks assume a branch is a trunk and others don't).
		isBranchFromTrunk := parentBranchName == defaultBranch
		parentState, err := meta.ReadBranchState(repo, parentBranchName, isBranchFromTrunk)
		if err != nil {
			return errors.WrapIf(err, "failed to read parent branch state")
		}

		// Create a new branch off of the parent
		logrus.WithFields(logrus.Fields{
			"parent":     parentBranchName,
			"new_branch": name,
		}).Debug("creating new branch from parent")
		if _, err := repo.CheckoutBranch(&git.CheckoutBranch{
			Name:      name,
			NewBranch: true,
		}); err != nil {
			return errors.WrapIff(err, "checkout error")
		}

		branchMeta := meta.Branch{
			Name:   name,
			Parent: parentState,
		}
		logrus.WithField("meta", branchMeta).Debug("writing branch metadata")
		if err := meta.WriteBranch(repo, branchMeta); err != nil {
			return errors.WrapIff(err, "failed to write av internal metadata for branch %q", name)
		}

		// If this isn't a new stack root, update the parent metadata to include
		// the new branch as a child.
		if !isBranchFromTrunk {
			parentMeta, _ := meta.ReadBranch(repo, parentBranchName)
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
