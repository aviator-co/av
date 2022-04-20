package stacks

import (
	"bytes"
	"emperror.dev/errors"
	"encoding/json"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/cleanup"
	"github.com/sirupsen/logrus"
)

type Branch struct {
	// The branch name associated with this stack.
	// Not serialized to JSON because the name can always be derived from the
	// name of the git ref.
	Name string `json:"-"`
	// The branch name associated with the parent of the stack (if any).
	Parent string `json:"parent"`
}

type BranchOpts struct {
	Name string
}

// CreateBranch creates a new stack branch based off of the current branch.
func CreateBranch(repo *git.Repo, opts *BranchOpts) (*Branch, error) {
	if opts.Name == "" {
		return nil, errors.New("new branch name is required")
	}

	var cu cleanup.Cleanup
	defer cu.Cleanup()

	parentHead, err := repo.HeadOid()
	if err != nil {
		return nil, errors.WrapIf(err, "failed to determine parent HEAD sha")
	}

	// Entering detached head state...
	// TODO:
	// 		This has unfortunate effect of breaking `git checkout -` to go back
	//		to the previous branch.
	resetCheckout, err := repo.CheckoutWithCleanup(parentHead)
	if err != nil {
		return nil, errors.WrapIff(err, "failed to checkout commit %s", parentHead)
	}
	cu.Add(resetCheckout)

	// Find the merge base.
	// If our history looks like:
	//     main: X -> Y -> Z
	//     pr1:  X -> 1a -> 1b
	// then the merge base is X.
	// TODO:
	//     This assumes that we're creating this stack branch from the current
	//     branch which is directly based on the base branch. We'll need to try
	//	   to read branch metadata to figure out the true parent branch if we're
	//     creating a branch with depth >= 3.
	mergeBase, err := repo.Git("merge-base", repo.DefaultBranch(), "HEAD")
	if err != nil {
		return nil, errors.WrapIff(
			err,
			"failed to determine merge base for commit %s and %s",
			git.ShortSha(parentHead), repo.DefaultBranch(),
		)
	}
	if mergeBase == "" {
		return nil, errors.New("merge base is empty")
	}
	logrus.WithField("commit", git.ShortSha(mergeBase)).Debug("determined merge base")

	// Create a squash commit by taking the tree of the HEAD commit and
	// parenting the commit on top of the merge base.
	// Using the sample history above, the squash commit would look like:
	//     av-base/<name>: X -> 1S
	// where 1S is the squash of 1a and 1b.
	// TODO:
	//     Make this commit message better. Should probably contain commit
	//     titles of each of the constituent commits.
	squashCommit, err := repo.Git("commit-tree", "HEAD^{tree}", "-p", mergeBase, "-m", "[[ av synthetic squash commit ]]")
	if err != nil {
		return nil, errors.WrapIff(
			err,
			"failed to create squash commit for branch",
		)
	}
	logrus.WithField("sha", git.ShortSha(squashCommit)).Debug("created synthetic squash commit")

	// Save the squash commit as a new branch.
	baseRefName := "refs/av/stack-base/" + opts.Name
	if err := repo.UpdateRef(&git.UpdateRef{Ref: baseRefName, New: squashCommit}); err != nil {
		return nil, err
	}
	logrus.WithField("ref", baseRefName).Debug("created base ref")

	// Create the new branch and check it out
	headRefName := "refs/heads/" + opts.Name
	if err := repo.UpdateRef(&git.UpdateRef{
		Ref: headRefName,
		New: squashCommit,
		Old: git.Missing,
	}); err != nil {
		logrus.WithError(err).Debug("failed to create new branch, it probably already exists")
		return nil, errors.Errorf("cannot create branch %s (does it already exist?)", opts.Name)
	}
	// Note: this can't include refs/head/... because that enters a detached
	// HEAD state.
	if _, err := repo.Git("checkout", opts.Name); err != nil {
		return nil, errors.WrapIff(err, "failed to checkout new branch %s", opts.Name)
	}

	// Finally, write the metadata and return the branch.
	next := &Branch{
		Name:   opts.Name,
		Parent: repo.DefaultBranch(),
	}
	if err := writeStackMetadata(repo, next); err != nil {
		return nil, err
	}
	cu.Cancel()
	return next, nil
}

func writeStackMetadata(repo *git.Repo, s *Branch) error {
	refName := "refs/av/stack-metadata/" + s.Name
	content, err := json.Marshal(s)
	if err != nil {
		return errors.Wrap(err, "failed to marshal stack metadata")
	}
	objectId, err := repo.GitStdin([]string{"hash-object", "-w", "--stdin"}, bytes.NewReader(content))
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
