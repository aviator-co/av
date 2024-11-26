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
var branchCmd = &cobra.Command{
	Use:   "branch [flags] <branch-name> [<parent-branch>]",
	Short: "Create or rename a branch in the stack",
	Long: strings.TrimSpace(`
Create a new branch that is stacked on the current branch.

<parent-branch>. If omitted, the new branch bases off the current branch.

If the --rename/-m flag is given, the current branch is renamed to the name
given as the first argument to the command. Branches should only be renamed
with this command (not with git branch -m ...) because av needs to update
internal tracking metadata that defines the order of branches within a stack.`),
	Args: cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) (reterr error) {
		if len(args) == 0 {
			// The only time we don't want to suppress the usage message is when
			// a user runs `av branch` with no arguments.
			return cmd.Usage()
		}

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
			stackBranchFlags.Parent = args[1]
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
		} else {
			var err error
			parentBranchName, err = repo.CurrentBranchName()
			if err != nil {
				return errors.WrapIff(err, "failed to get current branch name")
			}
		}

		remoteName := repo.GetRemoteName()
		if parentBranchName == remoteName+"/HEAD" {
			parentBranchName = defaultBranch
		}
		parentBranchName = strings.TrimPrefix(parentBranchName, remoteName+"/")

		isBranchFromTrunk, err := repo.IsTrunkBranch(parentBranchName)
		if err != nil {
			return errors.WrapIf(err, "failed to determine if branch is a trunk")
		}
		checkoutStartingPoint := parentBranchName
		var parentHead string
		if isBranchFromTrunk {
			// If the parent is trunk, start from the remote tracking branch.
			checkoutStartingPoint = remoteName + "/" + defaultBranch
			// If the parent is the trunk, we don't log the parent branch's head
			parentHead = ""
		} else {
			var err error
			parentHead, err = repo.RevParse(&git.RevParse{Rev: parentBranchName})
			if err != nil {
				return errors.WrapIff(
					err,
					"failed to determine head commit of branch %q",
					parentHead,
				)
			}

			if _, exist := tx.Branch(parentBranchName); !exist {
				return errParentNotAdopted
			}
		}

		// Resolve to a commit hash for the starting point.
		//
		// Different people have different setups, and they affect how they use
		// branch.<name>.merge. Different tools have different ways to interpret this
		// config. Some people want to set it to the same name only when it's pushed. Some
		// people want to set it to none. etc. etc.
		//
		// For this new ref creation specifically, git automatically guesses what to set for
		// branch.<name>.merge. For now, by using a commit hash, we can suppress all of
		// those behaviors. Later maybe we can add an av-cli config to control what to set
		// for branch.<name>.merge at what timing.
		startPointCommitHash, err := repo.RevParse(&git.RevParse{Rev: checkoutStartingPoint})
		if err != nil {
			return errors.WrapIf(err, "failed to determine commit hash of starting point")
		}

		// Create a new branch off of the parent
		if _, err := repo.CheckoutBranch(&git.CheckoutBranch{
			Name:       branchName,
			NewBranch:  true,
			NewHeadRef: startPointCommitHash,
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

		cu.Cancel()
		if err := tx.Commit(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	branchCmd.Flags().
		StringVar(&stackBranchFlags.Parent, "parent", "", "the parent branch to base the new branch off of")
	// NOTE: We use -m as the shorthand here to match `git branch -m ...`.
	// See the comment on stackBranchFlags.Rename.
	branchCmd.Flags().
		BoolVarP(&stackBranchFlags.Rename, "rename", "m", false, "rename the current branch")
	branchCmd.Flags().
		BoolVar(&stackBranchFlags.Force, "force", false, "force rename the current branch, even if a pull request exists")

	_ = branchCmd.RegisterFlagCompletionFunc(
		"parent",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			branches, _ := allBranches()
			return branches, cobra.ShellCompDirectiveDefault
		},
	)
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
