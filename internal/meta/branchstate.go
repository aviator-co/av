package meta

import (
	"encoding/json"
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
	// stacks can target for merge. Usually, the only trunk branch for a repository is main or
	// master.
	Trunk bool `json:"trunk,omitempty"`

	// The branching point commit hash.
	//
	// When we start a new branch off of a parent branch, we record the
	// commit SHA of the parent's latest commit at that time as the
	// branching point commit hash. This allows us to later identify which
	// commits belong to the child branch when syncing with the parent
	// branch.
	//
	// NOTE: This field is named "head" in the JSON for historical reasons.
	//
	// This field may be empty if the branching off from a trunk branch. In
	// that case, we will find the branching point commit hash based on `git
	// merge-base`. Note that this will only work if the trunk branch has
	// not been rebased since the branch was created, which typically
	// stands. On the other hand, when branching off of a non-trunk branch,
	// we should almost always set this field as the non-trunk branches are
	// tend to be rebased.
	BranchingPointCommitHash string `json:"head,omitempty"`
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
		return BranchState{Name: s, BranchingPointCommitHash: ""}, nil
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
