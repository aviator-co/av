package meta

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type Branch struct {
	// The branch name associated with this stack.
	// Not stored in JSON because the name can always be derived from the name
	// of the git ref.
	Name string `json:"-"`

	// Information about the parent branch.
	Parent BranchState `json:"parent,omitempty"`

	// The children branches of this branch within the stack (if any).
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
	// 	  effectively erases the UnmorshalJSON method (otherwise we get a stack
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

	// Everything except name (since that is set externally) and parent is set
	// on the actual Branch object we're unmarshalling into
	b.Children = d.Children
	b.PullRequest = d.PullRequest

	// Parse the parent information (which can either be a string or a JSON)
	var err error
	b.Parent, err = unmarshalBranchState(d.Parent)
	if err != nil {
		return err
	}
	// can't do this because sometimes we read an uninitialized branch
	//if b.Parent.Name == "" {
	//	return errors.Errorf("cannot unmarshal Branch from JSON: parent branch of %q is unset", b.Name)
	//}

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

func unmarshalBranch(repo *git.Repo, name string, refName string, blob string) (Branch, bool) {
	branch := Branch{Name: name}
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

// ReadBranch loads information about the branch from the git repository.
// Returns the branch metadata and a boolean indicating if the branch metadata
// already existed and was loaded. If the branch metadata does not exist, a
// useful default is returned.
func ReadBranch(repo *git.Repo, branchName string) (Branch, bool) {
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
		return Branch{
			Name: branchName,
			Parent: BranchState{
				Trunk: true,
				Name:  defaultBranch,
			},
		}, false
	}

	return unmarshalBranch(repo, branchName, refName, blob)
}

// ReadAllBranches fetches all branch metadata stored in the git repository.
// It returns a map where the key is the name of the branch.
func ReadAllBranches(repo *git.Repo) (map[string]Branch, error) {
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
	branches := make(map[string]Branch, len(refs))
	for _, ref := range refContents {
		name := strings.TrimPrefix(ref.Revision, branchMetaRefPrefix)
		branch, _ := unmarshalBranch(repo, name, ref.Revision, string(ref.Contents))
		branches[name] = branch
	}
	return branches, nil
}

// Find all the ancestor branches of the given branch name and append them to
// the given slice (in topological order: a comes before b if a is an ancestor
// of b).
func PreviousBranches(branches map[string]Branch, name string) ([]string, error) {
	current, ok := branches[name]
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
	previous, err := PreviousBranches(branches, parent.Name)
	if err != nil {
		return nil, err
	}
	return append(previous, parent.Name), nil
}

// Find all the child branches of the given branch name and append them to
// the given slice (in topological order: a comes before b if a is an ancestor
// of b).
func SubsequentBranches(branches map[string]Branch, name string) ([]string, error) {
	logrus.Debugf("finding subsequent branches for %q", name)
	var res []string
	branchMeta, ok := branches[name]
	if !ok {
		return nil, fmt.Errorf("branch metadata not found for %q", name)
	}
	if len(branchMeta.Children) == 0 {
		return res, nil
	}
	for _, child := range branchMeta.Children {
		res = append(res, child)
		desc, err := SubsequentBranches(branches, child)
		if err != nil {
			return nil, err
		}
		res = append(res, desc...)
	}

	return res, nil
}

func FindStackRoot(branches map[string]Branch, name string) (Branch, bool) {
	branchMeta, ok := branches[name]
	if !ok {
		return Branch{}, false
	}
	if branchMeta.Parent.Trunk {
		return branchMeta, true
	}
	return FindStackRoot(branches, branchMeta.Parent.Name)
}

// WriteBranch writes branch metadata to the git repository.
// It can be loaded again with ReadBranch.
func WriteBranch(repo *git.Repo, s Branch) error {
	// Assert a few invariants here
	// These should be checked by the caller before calling WriteBranch, but
	// we want to be extra safe to avoid getting into an inconsistent state.
	if s.Name == "" {
		return errors.New("cannot write branch metadata: branch name is empty")
	}

	if s.Parent.Name == s.Name {
		return errors.New("cannot write branch metadata: parent branch is the same as the branch itself")
	}

	if s.Parent.Trunk && s.Parent.Head != "" {
		return errors.New("invariant error: cannot write branch metadata: parent branch is a trunk branch and has a head commit assigned")
	} else if !s.Parent.Trunk && s.Parent.Head == "" {
		return errors.New("invariant error: cannot write branch metadata: parent branch is not a trunk branch and has no head commit assigned")
	}

	if slices.Contains(s.Children, s.Name) {
		return errors.New("cannot write branch metadata: branch is a child of itself")
	}

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

func DeleteBranch(repo *git.Repo, name string) error {
	refName := branchMetaRefName(name)
	if err := repo.UpdateRef(&git.UpdateRef{Ref: refName, New: git.Missing}); err != nil {
		return err
	}
	logrus.WithField("ref", refName).Debug("deleted branch metadata")
	return nil
}

const branchMetaRefPrefix = "refs/av/branch-metadata/"

func branchMetaRefName(branchName string) string {
	return branchMetaRefPrefix + branchName
}
