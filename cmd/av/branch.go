package main

import (
	"context"
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
	// If true, split the latest commit into a new branch
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
		ctx := cmd.Context()
		if len(args) == 0 && !branchFlags.Split {
			// The only time we don't want to suppress the usage message is when
			// a user runs `av branch` with no arguments.
			return cmd.Usage()
		}

		repo, err := getRepo(ctx)
		if err != nil {
			return err
		}

		db, err := getDB(ctx, repo)
		if err != nil {
			return err
		}
		var branchName string
		if len(args) == 0 && branchFlags.Split {
			branchName = ""
		} else {
			branchName = args[0]
		}

		if branchFlags.Rename {
			return branchMove(ctx, repo, db, branchName, branchFlags.Force)
		}
		if branchFlags.Split {
			status, err := repo.Status(ctx)
			if err != nil {
				return errors.Errorf("cannot get the status of the repository: %v", err)
			}
			currentBranchName := status.CurrentBranch
			currentCommitOID := status.OID
			if currentCommitOID == "" {
				fmt.Fprint(
					os.Stderr,
					colors.Failure("The repository is in an initial state with no commits."),
				)
				return errors.New("the repository has no commits")
			}

			err = branchSplit(ctx, repo, db, currentBranchName, branchName)
			if err != nil {
				return errors.Errorf("split failed: %v", err)
			}
			return nil
		}

		if len(args) == 2 {
			branchFlags.Parent = args[1]
		}

		return createBranch(ctx, repo, db, branchName, branchFlags.Parent)
	},
}

func validateBranchIsTip(stackNext stackNextModel) error {
	atTip, err := isAtTipOfStack(stackNext)
	if err != nil {
		return fmt.Errorf("error determining stack tip: %w", err)
	}

	if !atTip {
		return fmt.Errorf("current branch is not at the tip of the stack")
	}

	return nil
}

func isAtTipOfStack(stackNext stackNextModel) (bool, error) {
	msg := stackNext.nextBranch()

	switch v := msg.(type) {
	case checkoutBranchMsg:
		// If it's a checkoutBranchMsg, it means there's no next branch available
		return true, nil
	case nextBranchMsg:
		// If it's a nextBranchMsg, it means there's a next branch
		return false, nil
	case error:
		// If it's an error, handle specific cases
		if strings.Contains(v.Error(), "already on last branch") {
			return true, nil
		}
		return false, v
	default:
		return false, fmt.Errorf("unexpected message type: %T", v)
	}
}

func branchSplit(
	ctx context.Context,
	repo *git.Repo,
	db meta.DB,
	currentBranchName string,
	newBranchName string,
) error {
	stackNext, err := newNextModel(ctx, false, 1)
	if err != nil {
		return fmt.Errorf("failed to initialize stack model: %w", err)
	}
	err = validateBranchIsTip(stackNext)
	if err != nil {
		fmt.Fprint(
			os.Stderr,
			colors.Failure("====================================================\n"),
			"The split operation was aborted.\n",
			colors.Failure(err.Error()+"\n"),
			colors.Failure("====================================================\n"),
		)
		return err
	}
	// Get the current branch reference
	currentBranchRefName := plumbing.NewBranchReferenceName(currentBranchName)
	currentBranchRef, err := repo.GoGitRepo().Reference(currentBranchRefName, true)
	if err != nil {
		return fmt.Errorf(
			"failed to get reference for current branch %s: %w",
			currentBranchName,
			err,
		)
	}

	lastCommitHash := currentBranchRef.Hash()
	// Get the last commit object
	lastCommit, err := repo.GoGitRepo().CommitObject(lastCommitHash)
	if err != nil {
		return fmt.Errorf("failed to get last commit: %w", err)
	}

	// Generate a branch name if none is provided
	if newBranchName == "" {
		newBranchName = sanitizeBranchName(lastCommit.Message)
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
	// Update current HEAD to the new branch
	if err := repo.GoGitRepo().Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, newBranchRefName)); err != nil {
		return fmt.Errorf("failed to update HEAD: %w", err)
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

	// Adopt new branch to av database
	err = adoptForceAdoption(ctx, repo, db, newBranchName, currentBranchName)
	if err != nil {
		return fmt.Errorf("failed to run adopt command: %w", err)
	}

	return nil
}

// sanitizeBranchName creates a valid branch name from a string.
func sanitizeBranchName(input string) string {
	sanitized := strings.ToLower(strings.TrimSpace(input))
	sanitized = strings.ReplaceAll(sanitized, " ", "-")
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "\\", "-")
	sanitized = strings.ReplaceAll(sanitized, ".", "-")
	sanitized = strings.ReplaceAll(sanitized, ":", "-")
	// Limit length for branch names
	if len(sanitized) > 40 {
		sanitized = sanitized[:40]
	}
	return sanitized
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
		BoolVar(&branchFlags.Split, "split", false, "split the last commit into a new branch, if no branch name is given, one will be auto-generated")

	_ = branchCmd.RegisterFlagCompletionFunc(
		"parent",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			branches, _ := allBranches(cmd.Context())
			return branches, cobra.ShellCompDirectiveNoSpace
		},
	)
}

func createBranch(
	ctx context.Context,
	repo *git.Repo,
	db meta.DB,
	branchName string,
	parentBranchName string,
) (reterr error) {
	// Determine important contextual information from Git
	// or if a parent branch is provided, check it allows as a default branch
	defaultBranch, err := repo.DefaultBranch(ctx)
	if err != nil {
		return errors.WrapIf(err, "failed to determine repository default branch")
	}

	tx := db.WriteTx()
	cu := cleanup.New(func() {
		logrus.WithError(reterr).Debug("aborting db transaction")
		tx.Abort()
	})
	defer cu.Cleanup()

	// Determine the parent branch and make sure it's checked out
	if parentBranchName == "" {
		var err error
		parentBranchName, err = repo.CurrentBranchName(ctx)
		if err != nil {
			return errors.WrapIff(err, "failed to get current branch name")
		}
	}

	remoteName := repo.GetRemoteName()
	if parentBranchName == remoteName+"/HEAD" {
		parentBranchName = defaultBranch
	}
	parentBranchName = strings.TrimPrefix(parentBranchName, remoteName+"/")

	isBranchFromTrunk, err := repo.IsTrunkBranch(ctx, parentBranchName)
	if err != nil {
		return errors.WrapIf(err, "failed to determine if branch is a trunk")
	}
	checkoutStartingPoint := parentBranchName
	var parentHead string
	if isBranchFromTrunk {
		// If the parent is trunk, start from the remote tracking branch.
		checkoutStartingPoint = remoteName + "/" + parentBranchName
		// If the parent is the trunk, we don't log the parent branch's head
		parentHead = ""
	} else {
		var err error
		parentHead, err = repo.RevParse(ctx, &git.RevParse{Rev: parentBranchName})
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
	// people want to set it to none. etc.
	//
	// For this new ref creation specifically, git automatically guesses what to set for
	// branch.<name>.merge. For now, by using a commit hash, we can suppress all of
	// those behaviors. Later maybe we can add an av-cli config to control what to set
	// for branch.<name>.merge at what timing.
	startPointCommitHash, err := repo.RevParse(ctx, &git.RevParse{Rev: checkoutStartingPoint})
	if err != nil {
		return errors.WrapIf(err, "failed to determine commit hash of starting point")
	}

	// Create a new branch off of the parent
	logrus.WithFields(logrus.Fields{
		"parent":     parentBranchName,
		"new_branch": branchName,
	}).Debug("creating new branch from parent")
	if _, err := repo.CheckoutBranch(ctx, &git.CheckoutBranch{
		Name:       branchName,
		NewBranch:  true,
		NewHeadRef: startPointCommitHash,
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
		if _, err := repo.CheckoutBranch(ctx, &git.CheckoutBranch{
			Name: parentBranchName,
		}); err != nil {
			logrus.WithError(err).Error("failed to return to original branch during cleanup")
		}
		if err := repo.BranchDelete(ctx, branchName); err != nil {
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

	cu.Cancel()
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func branchMove(
	ctx context.Context,
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
		oldBranch, err = repo.CurrentBranchName(ctx)
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
		defaultBranch, err := repo.DefaultBranch(ctx)
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
	if ok, err := repo.DoesBranchExist(ctx, oldBranch); err != nil {
		return err
	} else if ok {
		if _, err := repo.Run(ctx, &git.RunOpts{
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
