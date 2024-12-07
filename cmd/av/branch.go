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
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var branchFlags struct {
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
	Split bool
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
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) (reterr error) {
		if len(args) == 0 && !branchFlags.Split {
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

		if branchFlags.Split {
			newBranchName := args[0]
			status, err := repo.Status()
			if err != nil {
				return errors.Errorf("cannot get the status of the repository: %v", err)
			}
			if !status.IsClean() {
				return errors.New(
					"The working directory is not clean. Please stash or commit changes before running 'av branch --split'.",
				)
			}
			currentBranchName := status.CurrentBranch
			currentCommitOID := status.OID
			if currentCommitOID == "" {
				return errors.New("the repository has no commits")
			}
			if err := splitLastCommit(repo, newBranchName); err != nil {
				fmt.Fprint(
					os.Stderr,
					colors.Failure("====================================================\n"),
					"The split operation was aborted.\n",
					"The original branch ",
					colors.UserInput(currentBranchName),
					" remains intact.\n",
					"Your Git repository is now in a detached HEAD state.\n",
					"To revert to your original branch and commit, run:\n",
					"    ",
					colors.CliCmd(fmt.Sprintf("git switch %s", currentBranchName)),
					"\n",
					colors.Failure("====================================================\n"),
				)
				return err
			}
			return nil

		}

		branchName := args[0]
		if branchFlags.Rename {
			return branchMove(repo, db, branchName, branchFlags.Force)
		}

		// Determine important contextual information from Git
		// or if a parent branch is provided, check it allows as a default branch
		defaultBranch, err := repo.DefaultBranch()
		if err != nil {
			return errors.WrapIf(err, "failed to determine repository default branch")
		}

		if len(args) == 2 {
			branchFlags.Parent = args[1]
		}

		tx := db.WriteTx()
		cu := cleanup.New(func() {
			logrus.WithError(reterr).Debug("aborting db transaction")
			tx.Abort()
		})
		defer cu.Cleanup()

		// Determine the parent branch and make sure it's checked out
		var parentBranchName string
		if branchFlags.Parent != "" {
			parentBranchName = branchFlags.Parent
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
		StringVar(&branchFlags.Parent, "parent", "", "the parent branch to base the new branch off of")
	// NOTE: We use -m as the shorthand here to match `git branch -m ...`.
	// See the comment on branchFlags.Rename.
	branchCmd.Flags().
		BoolVarP(&branchFlags.Rename, "rename", "m", false, "rename the current branch")
	branchCmd.Flags().
		BoolVar(&branchFlags.Force, "force", false, "force rename the current branch, even if a pull request exists")
	branchCmd.Flags().
		BoolVar(&branchFlags.Split, "split", false, "split the last commit into a new branch, if no branch name provided we will generate one")

	branchCmd.MarkFlagsMutuallyExclusive("split", "parent")
	branchCmd.MarkFlagsMutuallyExclusive("split", "rename")

	_ = branchCmd.RegisterFlagCompletionFunc(
		"parent",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			branches, _ := allBranches()
			return branches, cobra.ShellCompDirectiveNoFileComp
		},
	)
}

func branchMove(
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
			fmt.Fprint(
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
		fmt.Fprint(
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

func splitLastCommit(repo *git.Repo, newBranchName string) error {
	// Create a new branch from the current HEAD

	// Get the current branch reference
	currentBranchRef, err := repo.GoGitRepo().Head()
	if err != nil {
		return fmt.Errorf("failed to get reference for current branch %w", err)
	}
	currentBranchRefName := currentBranchRef.Name()

	lastCommitHash := currentBranchRef.Hash()
	// Get the last commit object
	lastCommit, err := repo.GoGitRepo().CommitObject(lastCommitHash)
	if err != nil {
		return fmt.Errorf("failed to get last commit: %w", err)
	}

	// If no branch name is provided, generate one from the last commit message
	if newBranchName == "" {
		newBranchName = branchNameFromMessage(lastCommit.Message)
		if newBranchName == "" {
			return errors.New(
				"Cannot create a valid branch name from the message, please provide a branch name",
			)
		}
	}

	// Get the parent of the last commit (HEAD~1)
	parentCommitIter := lastCommit.Parents()
	parentCommit, err := parentCommitIter.Next()
	if err != nil {
		return fmt.Errorf("failed to get parent commit: %w", err)
	}

	// Create a new branch pointing to the last commit
	newBranchRefName := plumbing.NewBranchReferenceName(newBranchName)
	newBranchRef := plumbing.NewHashReference(newBranchRefName, lastCommitHash)
	if err := repo.GoGitRepo().Storer.SetReference(newBranchRef); err != nil {
		return fmt.Errorf("failed to create new branch: %w", err)
	}

	updatedCurrentBranchRef := plumbing.NewHashReference(currentBranchRefName, parentCommit.Hash)
	if err := repo.GoGitRepo().Storer.SetReference(updatedCurrentBranchRef); err != nil {
		return fmt.Errorf("failed to update current branch: %w", err)
	}

	fmt.Fprint(
		os.Stdout,
		colors.Success(
			fmt.Sprintf(
				"Successfully split the last commit into a new branch %s.\n",
				newBranchName,
			),
		),
	)

	return nil
}
