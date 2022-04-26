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
	refContents, err := repo.CatFileBatch(&git.CatFileBatch{
		Revisions: refNames,
	})
	if err != nil {
		return nil, err
	}

	metas := make(map[string]*BranchMetadata)
	children := make(map[string][]string)
	hasParent := make(map[string]bool)
	for _, ref := range refContents {
		var meta BranchMetadata
		meta.Name = strings.TrimPrefix(ref.Revision, "refs/av/stack-metadata/")
		if err := json.Unmarshal(ref.Contents, &meta); err != nil {
			return nil, errors.WrapIff(err, "failed to unmarshal metadata for branch %q", ref.Revision)
		}
		metas[meta.Name] = &meta
		hasParent[meta.Name] = true
		// set default value for parent
		hasParent[meta.Parent] = hasParent[meta.Parent]
		children[meta.Parent] = append(children[meta.Parent], meta.Name)
	}

	trees := make(map[string]*Tree)
	// Iterate over all the stack roots and construct a tree for each
	for branch, hasParent := range hasParent {
		if hasParent {
			continue
		}
		metas[branch] = &BranchMetadata{
			Name: branch,
		}
		trees[branch] = completeTree(branch, children, metas)
	}

	return trees, nil
}

func completeTree(branchName string, children map[string][]string, branches map[string]*BranchMetadata) *Tree {
	tree := &Tree{
		Branch: branches[branchName],
	}
	for _, child := range children[branchName] {
		childTree := completeTree(child, children, branches)
		childTree.Previous = tree
		tree.Next = append(tree.Next, childTree)
	}
	return tree
}
