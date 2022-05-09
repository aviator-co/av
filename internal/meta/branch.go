package meta

import (
	"bytes"
	"emperror.dev/errors"
	"encoding/json"
	"github.com/aviator-co/av/internal/git"
	"github.com/sirupsen/logrus"
)

type Branch struct {
	// The branch name associated with this stack.
	// Not stored in JSON because the name can always be derived from the name
	// of the git ref.
	Name string `json:"-"`
	// The branch name associated with the parent of the stack (if any).
	Parent string `json:"parent"`

	PullRequest PullRequest `json:"pullRequest,omitempty"`
}

type PullRequest struct {
	ID        string `json:"id"`
	Number    int64  `json:"number"`
	Permalink string `json:"permalink"`
}

// GetBranch loads information about the branch from the git repository.
// Returns the branch metadata and a boolean indicating if the branch metadata
// already existed and was loaded. If the branch metadata does not exist, a
// useful default is returned.
func GetBranch(repo *git.Repo, branchName string) (Branch, bool) {
	// No matter what, we return something useful.
	// We have to set name since it's not loaded from the JSON blob.
	var branch Branch
	branch.Name = branchName

	refName := branchMetaRefName(branchName)
	blob, err := repo.Git("cat-file", "blob", refName)

	// Just assume that any error here means that the metadata ref doesn't exist
	// (there's no easy way to distinguish between that and an actual Git error)
	if err != nil {
		return branch, false
	}

	if err := json.Unmarshal([]byte(blob), &branch); err != nil {
		logrus.WithError(err).WithField("ref", refName).Error("corrupt stack metadata, deleting...")
		_ = repo.UpdateRef(&git.UpdateRef{Ref: refName, New: git.Missing})
		return branch, false
	}

	return branch, true
}

func WriteBranch(repo *git.Repo, s Branch) error {
	refName := branchMetaRefName(s.Name)
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

func branchMetaRefName(branchName string) string {
	return "refs/av/stack-metadata/" + branchName
}
