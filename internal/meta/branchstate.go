package meta

import (
	"emperror.dev/errors"
	"encoding/json"
	"github.com/aviator-co/av/internal/git"
	"github.com/sirupsen/logrus"
)

type BranchState struct {
	// The branch name associated with the parent of the stack (if any).
	// If empty, this branch (potentially*) is considered a stack root.
	// (*depending on the context, we only consider the branch a stack root if
	// it also has children branches; for example, any "vanilla" branch off of
	// trunk will have no parent, but we usually don't explicitly consider it a
	// stack unless it also has stack children)
	Name string `json:"name"`

	// If true, consider the branch a trunk branch. A trunk branch is one that
	// that stacks can target for merge. Usually, the only trunk branch for a
	// repository is main or master.
	Trunk bool `json:"trunk,omitempty"`

	// The commit SHA of the parent's latest commit. This is used when syncing
	// the branch with the parent to identify the commits that belong to the
	// child branch (since the HEAD of the parent branch may change).
	// This will be unset if Trunk is true.
	Head string `json:"head,omitempty"`
}

func ReadBranchState(repo *git.Repo, branch string, trunk bool) (BranchState, error) {
	if trunk {
		return BranchState{
			Name:  branch,
			Trunk: true,
		}, nil
	}

	head, err := repo.RevParse(&git.RevParse{Rev: "refs/heads/" + branch})
	if err != nil {
		return BranchState{}, errors.WrapIff(err, "failed to determine HEAD for branch %q", branch)
	}
	return BranchState{
		Name: branch,
		Head: head,
	}, nil
}

// BaseCommit determines the base commit for the given branch.
// The base commit is defined as the latest commit on the branch that should not
// be considered one of the critical commits on the stacked branch itself.
// This is essentially the merge-base of the branch and its parent and should be
// used as the `<upstream>` in the `git rebase --onto <parent-branch> <upstream>`
// command.
func (b Branch) BaseCommit(r *git.Repo) (string, error) {
	// For non-root stacked branches, we always store the head commit in the
	// metadata.
	if b.Parent.Head != "" {
		return b.Parent.Head, nil
	}

	// Otherwise, this branch is a stack root (and the parent is a trunk branch)
	// so we just need to determine the merge base. The critical assumption here
	// is that commits in trunk branches are never modified (i.e., rebased).
	if !b.Parent.Trunk {
		// COMPAT:
		// This shouldn't happen for any branch created after this commit is
		// introduced, but we don't want to completely barf for branches that
		// were already created.
		logrus.Warnf(
			"invariant error: corrupt stack metadata: "+
				"branch %q parent %q should have (head XOR trunk) set "+
				"(this may result in incorrect rebases)",
			b.Name, b.Parent.Name,
		)
	}

	base, err := r.MergeBase(&git.MergeBase{
		Revs: []string{b.Name, b.Parent.Name},
	})
	if err != nil {
		return "", errors.WrapIff(err, "failed to determine merge base for branch %q and %q", b.Name, b.Parent.Name)
	}
	return base, nil
}

// unmarshalBranchState unmarshals a BranchState from JSON (which can either be
// a string value or a JSON object).
func unmarshalBranchState(data []byte) (BranchState, error) {
	// COMPAT: If the parent is unset/null/empty, it means that the branch is
	// a stack root and so the parent branch is considered a trunk.
	if len(data) == 0 || string(data) == `null` || string(data) == `""` {
		return BranchState{Trunk: true}, nil
	}

	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return BranchState{}, err
		}
		if s == "" {
			return BranchState{}, nil
		}
		return BranchState{Name: s, Head: ""}, nil
	}

	// Define a type alias here so that it doesn't have the UnmarshalJSON
	// method (otherwise we get a recursive infinite loop).
	type alias BranchState
	var s alias
	if err := json.Unmarshal(data, &s); err != nil {
		return BranchState{}, err
	}
	return BranchState(s), nil
}
