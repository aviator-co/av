package main

import (
	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/cleanup"
	"github.com/aviator-co/av/internal/utils/sliceutils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var stackBranchFlags struct {
	// The parent branch to base the new branch off.
	// By default, this is the current branch.
	Parent string
	// If true, rename the current branch ("move" in Git parlance, though we
	// avoid that language here since we're not changing the branch's position
	// within the stack).
	Rename bool
}
var stackBranchCmd = &cobra.Command{
	Use:   "branch [flags] <branch-name>",
	Short: "create a new stacked branch",
	Long: `Create a new branch that is stacked on the current branch.

If the --rename/-m flag is given, the current branch is renamed to the name
given as the first argument to the command. Branches should only be renamed
with this command (not with git branch -m ...) because av needs to update
internal tracking metadata that defines the order of branches within a stack.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) (reterr error) {
		if len(args) != 1 {
			_ = cmd.Usage()
			return errors.New("exactly one branch name is required")
		}
		branchName := args[0]

		repo, err := getRepo()
		if err != nil {
			return err
		}

		db, err := getDB(repo)
		if err != nil {
			return err
		}

		if stackBranchFlags.Rename {
			return stackBranchMove(repo, db, branchName)
		}

		tx := db.WriteTx()
		cu := cleanup.New(func() {
			logrus.WithError(reterr).Debug("aborting db transaction")
			tx.Abort()
		})
		defer cu.Cleanup()

		// Determine important contextual information from Git
		defaultBranch, err := repo.DefaultBranch()
		if err != nil {
			return errors.WrapIf(err, "failed to determine repository default branch")
		}

		// Determine the parent branch and make sure it's checked out
		var parentBranchName string
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
		var parentHead string
		if !isBranchFromTrunk {
			var err error
			parentHead, err := repo.RevParse(&git.RevParse{Rev: parentBranchName})
			if err != nil {
				return errors.WrapIff(err, "failed to determine head commit of branch %q", parentHead)
			}
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

		cu.Cancel()
		if err := tx.Commit(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	stackBranchCmd.Flags().StringVar(&stackBranchFlags.Parent, "parent", "", "the parent branch to base the new branch off of")
	// NOTE: We use -m as the shorthand here to match `git branch -m ...`.
	// See the comment on stackBranchFlags.Rename.
	stackBranchCmd.Flags().BoolVarP(&stackBranchFlags.Rename, "rename", "m", false, "rename the current branch")
}

func stackBranchMove(
	repo *git.Repo,
	db meta.DB,
	newBranch string,
) (reterr error) {
	oldBranch, err := repo.CurrentBranchName()
	if err != nil {
		return err
	}

	tx := db.WriteTx()
	cu := cleanup.New(func() {
		logrus.WithError(reterr).Debug("aborting db transaction")
		tx.Abort()
	})
	defer cu.Cleanup()

	if oldBranch == newBranch {
		return errors.Errorf("cannot rename branch to itself")
	}

	currentMeta, _ := tx.Branch(oldBranch)
	currentMeta.Name = newBranch
	tx.DeleteBranch(oldBranch)
	tx.SetBranch(currentMeta)

	// Update the parent's reference to the child (unless the parent is a trunk
	// which doesn't maintain references to children).
	if !currentMeta.Parent.Trunk {
		parentMeta, _ := tx.Branch(currentMeta.Parent.Name)
		sliceutils.Replace(parentMeta.Children, oldBranch, newBranch)
		tx.SetBranch(parentMeta)
	}

	// Update all child branches to refer to the correct (renamed) parent.
	for _, child := range currentMeta.Children {
		childMeta, _ := tx.Branch(child)
		childMeta.Parent.Name = newBranch
		tx.SetBranch(childMeta)
	}

	// Finally, actually rename the branch in Git
	if _, err := repo.Run(&git.RunOpts{
		Args:      []string{"branch", "-m", newBranch},
		ExitError: true,
	}); err != nil {
		return errors.WrapIff(err, "failed to rename Git branch")
	}

	cu.Cancel()
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
