package treedetector

import (
	"emperror.dev/errors"
	avgit "github.com/aviator-co/av/internal/git"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const iterStopErr = errors.Sentinel("stop")

type BranchPiece struct {
	Name plumbing.ReferenceName

	// If any of these are set, Parent, ParentIsTrunk, ParentMergeBase, and IncludedCommits are
	// unset.
	PossibleParents     []plumbing.ReferenceName
	ContainsMergeCommit bool

	Parent          plumbing.ReferenceName
	ParentIsTrunk   bool
	ParentMergeBase plumbing.Hash
	IncludedCommits []*object.Commit
}

func DetectBranches(
	repo *avgit.Repo,
	unmanagedBranches []plumbing.ReferenceName,
) (map[plumbing.ReferenceName]*BranchPiece, error) {
	hashToRefMap, refToHashMap, err := getBranchHashes(repo.GoGitRepo())
	if err != nil {
		return nil, err
	}

	ret := map[plumbing.ReferenceName]*BranchPiece{}
	for _, bn := range unmanagedBranches {
		currentHash := refToHashMap[bn]
		nearestTrunkCommit, err := getNearestTrunkCommit(repo, bn)
		if err != nil {
			return nil, err
		}
		if currentHash == nearestTrunkCommit {
			// This branch is currently on the trunk or already merged to trunk. We
			// don't have to adopt it.
			continue
		}
		bp, err := traverseUntilTrunk(repo, bn, nearestTrunkCommit, hashToRefMap, refToHashMap)
		if err != nil {
			return nil, err
		}
		ret[bn] = bp
	}
	return ret, nil
}

func traverseUntilTrunk(
	repo *avgit.Repo,
	branch plumbing.ReferenceName,
	nearestTrunkCommit plumbing.Hash,
	hashToRefMap map[plumbing.Hash][]plumbing.ReferenceName,
	refToHashMap map[plumbing.ReferenceName]plumbing.Hash,
) (*BranchPiece, error) {
	commit, err := repo.GoGitRepo().CommitObject(refToHashMap[branch])
	if err != nil {
		return nil, err
	}
	ret := &BranchPiece{
		Name: branch,
	}
	// Do a commit traversal. We can stop the traversal when we hit the trunk or if we find a
	// commit that has multiple parents.
	err = object.NewCommitPreorderIter(commit, nil, nil).ForEach(func(c *object.Commit) error {
		if c.Hash == nearestTrunkCommit {
			trunk, err := repo.DefaultBranch()
			if err != nil {
				return err
			}
			ret.Parent = plumbing.NewBranchReferenceName(trunk)
			ret.ParentIsTrunk = true
			ret.ParentMergeBase = c.Hash
			return iterStopErr
		}
		if c.NumParents() > 1 {
			ret.ContainsMergeCommit = true
			return iterStopErr
		}
		if c.Hash != commit.Hash {
			if parents, ok := hashToRefMap[c.Hash]; ok {
				if len(parents) > 1 {
					ret.PossibleParents = parents
					return iterStopErr
				}
				ret.Parent = parents[0]
				ret.ParentIsTrunk = false
				ret.ParentMergeBase = c.Hash
				return iterStopErr
			}
		}
		ret.IncludedCommits = append(ret.IncludedCommits, c)
		return nil
	})
	if err != nil && err != iterStopErr {
		return nil, err
	}
	return ret, nil
}

func getNearestTrunkCommit(
	repo *avgit.Repo,
	ref plumbing.ReferenceName,
) (plumbing.Hash, error) {
	trunk, err := repo.DefaultBranch()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	// TODO(draftcode): Check if the branch exists. Use the rtb as well.

	mbArgs := []string{ref.String(), trunk}
	// Per git-merge-base(1), this should return the nearest commits from HEAD among the
	// the trunk branches since we don't specify --octopus.
	mb, err := repo.MergeBase(mbArgs...)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return plumbing.NewHash(mb), nil
}

func getBranchHashes(
	repo *git.Repository,
) (map[plumbing.Hash][]plumbing.ReferenceName, map[plumbing.ReferenceName]plumbing.Hash, error) {
	hashToRefMap := map[plumbing.Hash][]plumbing.ReferenceName{}
	refToHashMap := map[plumbing.ReferenceName]plumbing.Hash{}

	brs, err := repo.Branches()
	if err != nil {
		return nil, nil, err
	}
	if err := brs.ForEach(func(ref *plumbing.Reference) error {
		hashToRefMap[ref.Hash()] = append(hashToRefMap[ref.Hash()], ref.Name())
		refToHashMap[ref.Name()] = ref.Hash()
		return nil
	}); err != nil {
		return nil, nil, err
	}
	return hashToRefMap, refToHashMap, nil
}
