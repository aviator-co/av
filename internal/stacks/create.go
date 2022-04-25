package stacks

import (
	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/sirupsen/logrus"
)

type CreateBranchOpts struct {
	Name string
}

// CreateBranch creates and checks out a new stack branch based off of the current branch.
func CreateBranch(repo *git.Repo, opts *CreateBranchOpts) error {
	// validate args
	if opts.Name == "" {
		return errors.New("new branch name is required")
	}

	// validate operation preconditions
	if _, err := repo.RevParse(&git.RevParse{Rev: opts.Name}); err == nil {
		return errors.Errorf("branch %q already exists", opts.Name)
	}

	// determine important contextual information from Git
	defaultBranch, err := repo.DefaultBranch()
	if err != nil {
		return errors.WrapIf(err, "failed to determine repository default branch")
	}
	parentBranch, err := repo.CurrentBranchName()
	if err != nil {
		return errors.WrapIff(err, "failed to get current branch name")
	}

	// create a new branch off of the parent
	if _, err := repo.CheckoutBranch(&git.CheckoutBranch{
		Name:      opts.Name,
		NewBranch: true,
	}); err != nil {
		logrus.WithError(err).Debugf("failed to checkout branch %q", opts.Name)
		return errors.Errorf(
			"failed to create branch %q (does it already exist?)",
			opts.Name,
		)
	}

	// If we're branching off of the default, we don't need to store any stack
	// metadata (since we don't actually consider it a stack until it has depth
	// >= 2).
	if parentBranch == defaultBranch {
		return nil
	}

	// Otherwise, we need to write the metadata that way we can construct the
	// DAG of the stack later.
	if err := writeMetadata(repo, &BranchMetadata{
		Name:   opts.Name,
		Parent: parentBranch,
	}); err != nil {
		return errors.WrapIff(err, "failed to write av internal metadata for branch %q", opts.Name)
	}
	return nil
}
