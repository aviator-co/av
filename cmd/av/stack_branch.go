package main

import (
	"fmt"
	"os"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/cleanup"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var stackBranchFlags struct {
	// The parent branch to base the new branch off.
	// By default, this is the current branch.
	Parent string
	// If true, rename the current branch ("move" in Git parlance, though we
	// avoid that language here since we're not changing the branch's position
	// within the stack). The branch can only be renamed if a pull request does
	// not exist.
	Rename bool
	// If true, rename the current branch even if a pull request exists.
	Force bool
}
var stackBranchCmd = &cobra.Command{
	Use:     "branch [flags] <branch-name> [<parent-branch>]",
	Aliases: []string{"b", "br"},
	Short:   "create a new stacked branch",
	Long: `Create a new branch that is stacked on the current branch.

<parent-branch>. If omitted, the new branch bases off the current branch.

If the --rename/-m flag is given, the current branch is renamed to the name
given as the first argument to the command. Branches should only be renamed
with this command (not with git branch -m ...) because av needs to update
internal tracking metadata that defines the order of branches within a stack.`,
	SilenceUsage: true,
	Args:         cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) (reterr error) {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		db, err := getDB(repo)
		if err != nil {
			return err
		}

		branchName := args[0]
		if stackBranchFlags.Rename {
			return stackBranchMove(repo, db, branchName, stackBranchFlags.Force)
		}

		// Determine important contextual information from Git
		// or if a parent branch is provided, check it allows as a default branch
		defaultBranch, err := repo.DefaultBranch()
		if err != nil {
			return errors.WrapIf(err, "failed to determine repository default branch")
		}

		if len(args) == 2 {
			parent := parseInputParentBranch(args[1])

			stackBranchFlags.Parent = parent
			isTrunk, err := repo.IsTrunkBranch(parent)
			if err != nil {
				return errors.WrapIf(err, "failed to check if the parent branch is trunk")
			}

			if isTrunk {
				defaultBranch = parent
			}
		}

		tx := db.WriteTx()
		cu := cleanup.New(func() {
			logrus.WithError(reterr).Debug("aborting db transaction")
			tx.Abort()
		})
		defer cu.Cleanup()

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

		isBranchFromTrunk, err := repo.IsTrunkBranch(parentBranchName)
		if err != nil {
			return errors.WrapIf(err, "failed to determine if branch is a trunk")
		}

		var parentHead string
		if !isBranchFromTrunk {
			var err error
			parentHead, err = repo.RevParse(&git.RevParse{Rev: parentBranchName})
			if err != nil {
				return errors.WrapIff(
					err,
					"failed to determine head commit of branch %q",
					parentHead,
				)
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
			parentMeta, ok := tx.Branch(parentBranchName)
			if !ok {
				// Handle case where the user created first branch by
				// `git switch -c` from trunk (i.e., created first branch
				// without using av), then wants to create a stacked branch
				// after it.
				parentMeta = meta.Branch{
					Name: parentBranchName,
					Parent: meta.BranchState{
						// Assume the parent is a branch from the default branch
						Name:  defaultBranch,
						Trunk: true,
					},
				}
			}
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
	stackBranchCmd.Flags().
		StringVar(&stackBranchFlags.Parent, "parent", "", "the parent branch to base the new branch off of")
	// NOTE: We use -m as the shorthand here to match `git branch -m ...`.
	// See the comment on stackBranchFlags.Rename.
	stackBranchCmd.Flags().
		BoolVarP(&stackBranchFlags.Rename, "rename", "m", false, "rename the current branch")
	stackBranchCmd.Flags().
		BoolVar(&stackBranchFlags.Force, "force", false, "force rename the current branch")

	_ = stackBranchCmd.RegisterFlagCompletionFunc("parent", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		branches, _ := allBranches()
		return branches, cobra.ShellCompDirectiveDefault
	})
}

func stackBranchMove(
	repo *git.Repo,
	db meta.DB,
	newBranch string,
	force bool,
) (reterr error) {
	c := strings.Count(newBranch, ":")
	if c > 1 {
		return errors.New("the branch name should be NEW_BRANCH or OLD_BRANCH:NEW_BRANCH")
	}

	var oldBranch string
	if strings.ContainsRune(newBranch, ':') {
		oldBranch, newBranch, _ = strings.Cut(newBranch, ":")
	} else {
		var err error
		oldBranch, err = repo.CurrentBranchName()
		if err != nil {
			return err
		}
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

	currentMeta, ok := tx.Branch(oldBranch)
	if !ok {
		defaultBranch, err := repo.DefaultBranch()
		if err != nil {
			return errors.WrapIf(err, "failed to determine repository default branch")
		}
		currentMeta.Parent = meta.BranchState{
			Name:  defaultBranch,
			Trunk: true,
		}
	}

	if !force {
		if currentMeta.PullRequest != nil {
			_, _ = fmt.Fprint(
				os.Stderr,
				colors.Failure(
					"Cannot rename branch ",
					currentMeta.Name,
					": pull request #",
					currentMeta.PullRequest.Number,
					" would be orphaned.\n",
				),
				colors.Faint("  - Use --force to override this check.\n"),
			)

			return actions.ErrExitSilently{ExitCode: 127}
		}
	}

	currentMeta.PullRequest = nil
	currentMeta.Name = newBranch
	tx.SetBranch(currentMeta)

	// Update all child branches to refer to the correct (renamed) parent.
	children := meta.Children(tx, oldBranch)
	for _, child := range children {
		child.Parent.Name = newBranch
		tx.SetBranch(child)
	}
	tx.DeleteBranch(oldBranch)

	// Finally, actually rename the branch in Git
	if ok, err := repo.DoesBranchExist(oldBranch); err != nil {
		return err
	} else if ok {
		if _, err := repo.Run(&git.RunOpts{
			Args:      []string{"branch", "-m", newBranch},
			ExitError: true,
		}); err != nil {
			return errors.WrapIff(err, "failed to rename Git branch")
		}
	} else {
		_, _ = fmt.Fprint(
			os.Stderr,
			"Branch ",
			colors.UserInput(oldBranch),
			" does not exist in Git. Updating av internal metadata only.\n",
		)
	}

	cu.Cancel()
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func parseInputParentBranch(branch string) string {
	// If the input parent branch is contained remote name, trim it
	return strings.Replace(branch, "origin/", "", 1)
}
