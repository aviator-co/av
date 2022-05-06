package stacks

import (
	"emperror.dev/errors"
	"encoding/json"
	"github.com/aviator-co/av/internal/git"
	"strings"
)

type Tree struct {
	Previous *Tree
	Next     []*Tree
	Branch   *BranchMetadata
}

type GetTreeOpts struct {
	Root string
}

var ErrNotInStack = errors.New("not currently in a stack")

// GetCurrentRoot determines the root of the current stack (if any).
// Returns ErrNotInStack if not currently checked out to a stacked branch,
// or a different error if one occurred.
func GetCurrentRoot(repo *git.Repo) (*Tree, error) {
	currentBranch, err := repo.CurrentBranchName()
	if err != nil {
		return nil, err
	}
	currentMeta := GetMetadata(repo, currentBranch)
	if currentMeta == nil {
		return nil, ErrNotInStack
	}

	var root string
	for currentMeta != nil {
		root = currentMeta.Parent
		currentMeta = GetMetadata(repo, currentMeta.Parent)
	}

	trees, err := GetTrees(repo)
	if err != nil {
		return nil, err
	}

	tree := trees[root]
	if tree == nil {
		return nil, errors.Errorf("invariant error: no tree found for stack root %q", root)
	}
	return tree, nil
}

func GetTrees(repo *git.Repo) (map[string]*Tree, error) {
	refs, err := repo.ListRefs(&git.ListRefs{
		Patterns: []string{"refs/av/stack-metadata/*"},
	})
	if err != nil {
		return nil, err
	}

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

	// To construct the trees, we need to find all the stack roots in the repo.
	// We do this by finding all the stack branches that do not themselves have
	// a parent branch. This is a two step process:
	// 1. Create maps (dicts) of all the relationships.
	branchMetadata := make(map[string]*BranchMetadata)
	branchChildren := make(map[string][]string)
	for _, ref := range refContents {
		var meta BranchMetadata
		meta.Name = strings.TrimPrefix(ref.Revision, "refs/av/stack-metadata/")
		if err := json.Unmarshal(ref.Contents, &meta); err != nil {
			return nil, errors.WrapIff(err, "failed to unmarshal metadata for branch %q", ref.Revision)
		}
		branchMetadata[meta.Name] = &meta
		branchChildren[meta.Parent] = append(branchChildren[meta.Parent], meta.Name)
	}

	// 2. Find all the branches that do not have a parent. These are the roots.
	trees := make(map[string]*Tree)
	for branch := range branchChildren {
		if meta := branchMetadata[branch]; meta != nil && meta.Parent != "" {
			continue
		}
		// Root branches don't actually have any associated branch metadata,
		// so we need to create a fake one.
		branchMetadata[branch] = &BranchMetadata{
			Name: branch,
		}
		trees[branch] = completeTree(branch, branchChildren, branchMetadata)
	}

	return trees, nil
}

func completeTree(
	branchName string,
	branchChildren map[string][]string,
	branches map[string]*BranchMetadata,
) *Tree {
	tree := &Tree{
		Branch: branches[branchName],
	}
	for _, child := range branchChildren[branchName] {
		childTree := completeTree(child, branchChildren, branches)
		childTree.Previous = tree
		tree.Next = append(tree.Next, childTree)
	}
	return tree
}
