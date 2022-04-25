package stacks

import (
	"bytes"
	"emperror.dev/errors"
	"encoding/json"
	"github.com/aviator-co/av/internal/git"
	"github.com/sirupsen/logrus"
)

type branchMetadata struct {
	// The branch name associated with this stack.
	// Not stored in JSON because the name can always be derived from the name
	// of the git ref.
	Name string `json:"-"`
	// The branch name associated with the parent of the stack (if any).
	Parent string `json:"parent"`
}

type CreateBranchOpts struct {
	Name string
}

// CreateBranch creates a new stack branch based off of the current branch.
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
	if err := writeStackMetadata(repo, &branchMetadata{
		Name:   opts.Name,
		Parent: parentBranch,
	}); err != nil {
		return errors.WrapIff(err, "failed to write av internal metadata for branch %q", opts.Name)
	}
	return nil
}

func stackMetadataRefName(branchName string) string {
	return "refs/av/stack-metadata/" + branchName
}

func writeStackMetadata(repo *git.Repo, s *branchMetadata) error {
	refName := stackMetadataRefName(s.Name)
	content, err := json.Marshal(s)
	if err != nil {
		return errors.Wrap(err, "failed to marshal stack metadata")
	}
	objectId, err := repo.GitStdin(
		[]string{"hash-object", "-w", "--stdin"},
		bytes.NewReader(content),
	)
	if err != nil {
		return errors.Wrap(err, "failed to store stack metadata in git")
	}
	if err := repo.UpdateRef(&git.UpdateRef{Ref: refName, New: objectId}); err != nil {
		return err
	}
	logrus.
		WithFields(logrus.Fields{"ref": refName, "sha": git.ShortSha(objectId)}).
		Debug("created stack ref")
	return nil
}

// readStackMetadata looks up the stack metadata for the given branch name.
// It returns the branchMetadata object or nil if it does not exist.
func readStackMetadata(repo *git.Repo, branchName string) *branchMetadata {
	refName := stackMetadataRefName(branchName)
	blob, err := repo.Git("cat-file", "blob", refName)

	// Just assume that any error here means that the metadata ref doesn't exist
	// (there's no easy way to distinguish between that and an actual Git error)
	if err != nil {
		return nil
	}

	var branch branchMetadata
	if err := json.Unmarshal([]byte(blob), &branch); err != nil {
		logrus.WithError(err).WithField("ref", refName).Error("corrupt stack metadata, deleting...")
		_ = repo.UpdateRef(&git.UpdateRef{Ref: refName, New: git.Missing})
		return nil
	}

	return &branch
}
