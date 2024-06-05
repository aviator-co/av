package meta

import (
	"encoding/json"

	"emperror.dev/errors"
	"github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type Branch struct {
	// The branch name associated with this stack.
	// Not stored in JSON because the name can always be derived from the name
	// of the git ref.
	Name string `json:"name"`

	// Information about the parent branch.
	Parent BranchState `json:"parent,omitempty"`

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
	State githubv4.PullRequestState `json:"state"`
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
		logrus.Fatalf(
			"invariant error: branch %q is its own parent (this is probably a bug with av)",
			name,
		)
	}
	previous, err := PreviousBranches(tx, parent.Name)
	if err != nil {
		return nil, err
	}
	return append(previous, parent.Name), nil
}

// SubsequentBranches finds all the child branches of the given branch name in
// "dependency order" (i.e., A comes before B if A is an ancestor of B).
// If the tree is not a straight line (which isn't explicitly supported!), the
// branches will be returned in depth-first traversal order.
func SubsequentBranches(tx ReadTx, name string) []string {
	logrus.Debugf("finding subsequent branches for %q", name)
	var res []string
	children := Children(tx, name)
	for _, child := range children {
		res = append(res, child.Name)
		res = append(res, SubsequentBranches(tx, child.Name)...)
	}
	return res
}

// StackBranches returns branches in the stack associated with the given branch.
func StackBranches(tx ReadTx, name string) ([]string, error) {
	root, found := Root(tx, name)
	if !found {
		return nil, errors.Errorf("branch %q is not in a stack", name)
	}

	var res = []string{root}
	res = append(res, SubsequentBranches(tx, root)...)
	return res, nil
}

// BranchesMap returns a map of branch names to their metadata.
func BranchesMap(tx ReadTx, names []string) (map[string]Branch, error) {
	branches := make(map[string]Branch, len(names))
	for _, branchName := range names {
		branch, ok := tx.Branch(branchName)
		if !ok {
			return nil, errors.Errorf("branch metadata not found for %q", branchName)
		}
		branches[branchName] = branch
	}
	return branches, nil
}

// Root determines the stack root of a branch.
func Root(tx ReadTx, name string) (string, bool) {
	for name != "" {
		branch, _ := tx.Branch(name)
		if branch.Parent.Trunk {
			return name, true
		}
		name = branch.Parent.Name
	}
	return "", false
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

// Children returns all the immediate children of the given branch.
func Children(tx ReadTx, name string) []Branch {
	branches := tx.AllBranches()
	var children []Branch
	for _, branch := range branches {
		if branch.Parent.Name == name {
			children = append(children, branch)
		}
	}
	// Sort for determinism.
	slices.SortFunc(children, func(a, b Branch) int {
		if a.Name < b.Name {
			return -1
		} else if a.Name > b.Name {
			return 1
		}
		return 0
	})
	return children
}

func ChildrenNames(tx ReadTx, name string) []string {
	branches := Children(tx, name)
	children := make([]string, 0, len(branches))
	for _, branch := range branches {
		children = append(children, branch.Name)
	}
	return children
}
