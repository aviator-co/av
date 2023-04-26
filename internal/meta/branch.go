package meta

import (
	"emperror.dev/errors"
	"encoding/json"
	"fmt"
	"github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"
)

type Branch struct {
	// The branch name associated with this stack.
	// Not stored in JSON because the name can always be derived from the name
	// of the git ref.
	Name string `json:"name"`

	// Information about the parent branch.
	Parent BranchState `json:"parent,omitempty"`

	// The child branches of this branch within the stack (if any).
	Children []string `json:"children,omitempty"`

	// The associated pull request information, if any.
	PullRequest *PullRequest `json:"pullRequest,omitempty"`

	// The merge commit onto the trunk branch, if any
	MergeCommit string `json:"mergeCommit,omitempty"`
}

func (b *Branch) IsStackRoot() bool {
	return b.Parent.Trunk
}

func (b *Branch) UnmarshalJSON(bytes []byte) error {
	// We have to do a bit of backwards-compatible trickery here to support the
	// fact that "parent" used to be a string field and now it's a struct
	// (the main reason it's a struct is because we want the parent info to be
	// updated atomically and doing it like this makes it harder to forget to
	// update the branch name but forget to update the HEAD sha).
	// Two things are happening here as far as the code is concerned:
	// 1. We want to still use the normal JSON machinery to parse most fields
	//    out of Branch (without having to write our own JSON parsing logic
	//    here). To do that, we have to define a type alias for Branch which
	// 	  effectively erases the UnmarshalJSON method (otherwise we get a stack
	//	  overflow as this function would be called recursively).
	// 2. We define a new type that embeds BranchAlias but overrides the Parent
	//    field so we can parse that manually ourselves.
	type BranchAlias Branch
	type data struct {
		BranchAlias
		Parent json.RawMessage `json:"parent"`
	}
	var d data
	if err := json.Unmarshal(bytes, &d); err != nil {
		return err
	}

	if b.Name != "" {
		d.BranchAlias.Name = b.Name
	}
	*b = Branch(d.BranchAlias)

	// Parse the parent information (which can either be a string or a JSON)
	var err error
	b.Parent, err = unmarshalBranchState(d.Parent)
	if err != nil {
		return err
	}

	logrus.Debugf("parsed branch metadata: %s => %#+v %#+v", bytes, d, b)
	return nil
}

var _ json.Unmarshaler = (*Branch)(nil)

type PullRequest struct {
	// The GitHub (GraphQL) ID of the pull request.
	ID string `json:"id"`
	// The pull request number.
	Number int64 `json:"number"`
	// The web URL for the pull request.
	Permalink string `json:"permalink"`
	// The state of the pull request (open, closed, or merged).
	State githubv4.PullRequestState
}

// GetNumber returns the number of the pull request or zero if the PullRequest is nil.
func (p *PullRequest) GetNumber() int64 {
	if p == nil {
		return 0
	}
	return p.Number
}

// PreviousBranches finds all the ancestor branches of the given branch name in
// "dependency order" (i.e., A comes before B if A is an ancestor of B).
func PreviousBranches(tx ReadTx, name string) ([]string, error) {
	current, ok := tx.Branch(name)
	if !ok {
		return nil, errors.Errorf("branch metadata not found for %q", name)
	}
	parent := current.Parent
	if parent.Trunk {
		return nil, nil
	}
	if parent.Name == name {
		logrus.Fatalf("invariant error: branch %q is its own parent (this is probably a bug with av)", name)
	}
	previous, err := PreviousBranches(tx, parent.Name)
	if err != nil {
		return nil, err
	}
	return append(previous, parent.Name), nil
}

// SubsequentBranches finds all the child branches of the given branch name in
// "dependency order" (i.e., A comes before B if A is an ancestor of B).
func SubsequentBranches(tx ReadTx, name string) ([]string, error) {
	logrus.Debugf("finding subsequent branches for %q", name)
	var res []string
	branchMeta, ok := tx.Branch(name)
	if !ok {
		return nil, fmt.Errorf("branch metadata not found for %q", name)
	}
	if len(branchMeta.Children) == 0 {
		return res, nil
	}
	for _, child := range branchMeta.Children {
		res = append(res, child)
		desc, err := SubsequentBranches(tx, child)
		if err != nil {
			return nil, err
		}
		res = append(res, desc...)
	}
	return res, nil
}

// Trunk determines the trunk of a branch.
func Trunk(tx ReadTx, name string) (string, bool) {
	for name != "" {
		branch, _ := tx.Branch(name)
		if branch.Parent.Trunk {
			return branch.Parent.Name, true
		}
		name = branch.Parent.Name
	}
	return "", false
}

//func FindStackRoot(branches map[string]Branch, name string) (Branch, bool) {
//	branchMeta, ok := branches[name]
//	if !ok {
//		return Branch{}, false
//	}
//	if branchMeta.Parent.Trunk {
//		return branchMeta, true
//	}
//	return FindStackRoot(branches, branchMeta.Parent.Name)
//}

//// WriteBranch writes branch metadata to the git repository.
//// It can be loaded again with ReadBranch.
//func WriteBranch(repo *git.Repo, s Branch) error {
//	// Assert a few invariants here
//	// These should be checked by the caller before calling WriteBranch, but
//	// we want to be extra safe to avoid getting into an inconsistent state.
//	if s.Name == "" {
//		return errors.New("cannot write branch metadata: branch name is empty")
//	}
//
//	if s.Parent.Name == s.Name {
//		return errors.New("cannot write branch metadata: parent branch is the same as the branch itself")
//	}
//
//	if s.Parent.Trunk && s.Parent.Head != "" {
//		return errors.New("invariant error: cannot write branch metadata: parent branch is a trunk branch and has a head commit assigned")
//	} else if !s.Parent.Trunk && s.Parent.Head == "" {
//		return errors.New("invariant error: cannot write branch metadata: parent branch is not a trunk branch and has no head commit assigned")
//	}
//
//	if slices.Contains(s.Children, s.Name) {
//		return errors.New("cannot write branch metadata: branch is a child of itself")
//	}
//
//	refName := branchMetaRefName(s.Name)
//	content, err := json.Marshal(s)
//	if err != nil {
//		return errors.Wrap(err, "failed to marshal stack metadata")
//	}
//	objectId, err := repo.GitStdin(
//		[]string{"hash-object", "-w", "--stdin"},
//		bytes.NewReader(content),
//	)
//	if err != nil {
//		return errors.Wrap(err, "failed to store stack metadata in git")
//	}
//	if err := repo.UpdateRef(&git.UpdateRef{Ref: refName, New: objectId}); err != nil {
//		return err
//	}
//	logrus.
//		WithFields(logrus.Fields{"ref": refName, "sha": git.ShortSha(objectId)}).
//		Debug("created stack ref")
//	return nil
//}
//
//func DeleteBranch(repo *git.Repo, name string) error {
//	refName := branchMetaRefName(name)
//	if err := repo.UpdateRef(&git.UpdateRef{Ref: refName, New: git.Missing}); err != nil {
//		return err
//	}
//	logrus.WithField("ref", refName).Debug("deleted branch metadata")
//	return nil
//}
