package refmeta

import (
	"encoding/json"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/sirupsen/logrus"
)

// ReadBranch loads information about the branch from the git repository.
// Returns the branch metadata and a boolean indicating if the branch metadata
// already existed and was loaded. If the branch metadata does not exist, a
// useful default is returned.
func ReadBranch(repo *git.Repo, branchName string) (meta.Branch, bool) {
	refName := branchMetaRefName(branchName)
	blob, err := repo.Git("cat-file", "blob", refName)

	// Just assume that any error here means that the metadata ref doesn't exist
	// (there's no easy way to distinguish between that and an actual Git error)
	if err != nil {
		defaultBranch, err := repo.DefaultBranch()
		if err != nil {
			// panic isn't great, but plumbing through the error is more effort
			// that it's worth here
			panic(errors.Wrap(err, "failed to determine repository default branch"))
		}
		// If there is no branch metadata, it probably means that they created
		// the branch with "git checkout -b" and we implicitly assume that
		// the branch is a stack root whose trunk is the repo default branch.
		return meta.Branch{
			Name: branchName,
			Parent: meta.BranchState{
				Trunk: true,
				Name:  defaultBranch,
			},
		}, false
	}

	return unmarshalBranch(repo, branchName, refName, blob)
}

// ReadAllBranches fetches all branch metadata stored in the git repository.
// It returns a map where the key is the name of the branch.
func ReadAllBranches(repo *git.Repo) (map[string]meta.Branch, error) {
	// Find all branch metadata ref names
	// Note: need `**` here (not just `*`) because Git seems to only match one
	// level of nesting in the ref pattern with just a single `*` (even though
	// the docs seem to suggest this to not be the case). With a single star,
	// we won't match branch names like `feature/add-xyz` or `travis/fix-123`.
	refs, err := repo.ListRefs(&git.ListRefs{
		Patterns: []string{branchMetaRefPrefix + "**"},
	})
	if err != nil {
		return nil, err
	}
	logrus.WithField("refs", refs).Debug("found branch metadata refs")

	// Read the contents of each ref to get the associated metadata blob...
	refNames := make([]string, len(refs))
	for i, ref := range refs {
		refNames[i] = ref.Name
	}
	refContents, err := repo.GetRefs(&git.GetRefs{
		Revisions: refNames,
	})
	if err != nil {
		return nil, err
	}

	// ...and for each metadata blob, parse it from JSON into a Branch
	branches := make(map[string]meta.Branch, len(refs))
	for _, ref := range refContents {
		name := strings.TrimPrefix(ref.Revision, branchMetaRefPrefix)
		branch, _ := unmarshalBranch(repo, name, ref.Revision, string(ref.Contents))
		branches[name] = branch
	}
	return branches, nil
}

const branchMetaRefPrefix = "refs/av/branch-metadata/"

func branchMetaRefName(branchName string) string {
	return branchMetaRefPrefix + branchName
}

func unmarshalBranch(repo *git.Repo, name string, refName string, blob string) (meta.Branch, bool) {
	branch := meta.Branch{Name: name}
	if err := json.Unmarshal([]byte(blob), &branch); err != nil {
		logrus.WithError(err).WithField("ref", refName).Error("corrupt stack metadata, deleting...")
		_ = repo.UpdateRef(&git.UpdateRef{Ref: refName, New: git.Missing})
		return branch, false
	}
	if branch.Parent.Name == "" {
		// COMPAT: assume parent branch is the default/mainline branch
		defaultBranch, err := repo.DefaultBranch()
		if err != nil {
			// panic isn't great, but plumbing through the error is more effort
			// that it's worth here
			panic(errors.Wrap(err, "failed to determine repository default branch"))
		}
		branch.Parent.Name = defaultBranch
		branch.Parent.Trunk = true
	}
	return branch, true
}
