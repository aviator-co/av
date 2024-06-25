package treedetector

import (
	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/utils/sliceutils"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

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

func DetectBranchTree(repo *git.Repository, remoteName string, trunkBranches []plumbing.ReferenceName) (map[plumbing.ReferenceName]*BranchPiece, error) {
	trunkCommits, err := getTrunkCommits(repo, remoteName, trunkBranches)
	if err != nil {
		return nil, err
	}
	branches, err := getBranchHashes(repo)
	if err != nil {
		return nil, err
	}

	d := &detector{
		repo:         repo,
		trunkCommits: trunkCommits,
		branches:     branches,
	}

	ret := map[plumbing.ReferenceName]*BranchPiece{}
	brs, err := repo.Branches()
	if err != nil {
		return nil, err
	}
	if err := brs.ForEach(func(ref *plumbing.Reference) error {
		if sliceutils.Contains(trunkBranches, ref.Name()) {
			return nil
		}
		bp, err := d.detectBranchTree(ref)
		if err != nil {
			return err
		}
		ret[ref.Name()] = bp
		return nil
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func getTrunkCommits(repo *git.Repository, remoteName string, trunkBranches []plumbing.ReferenceName) (map[plumbing.ReferenceName][]*object.Commit, error) {
	remote, err := repo.Remote(remoteName)
	if err != nil {
		return nil, errors.Errorf("failed to get remote %q: %v", remoteName, err)
	}

	ret := map[plumbing.ReferenceName][]*object.Commit{}
	for _, bn := range trunkBranches {
		ref, err := repo.Reference(bn, true)
		if err != nil && err != plumbing.ErrReferenceNotFound {
			return nil, err
		}
		if ref != nil {
			commit, err := repo.CommitObject(ref.Hash())
			if err != nil {
				return nil, err
			}
			ret[bn] = append(ret[bn], commit)
		}

		rtb := mapToRemoteTrackingBranch(remote.Config(), bn)
		if rtb != nil {
			ref, err = repo.Reference(*rtb, true)
			if err != nil && err != plumbing.ErrReferenceNotFound {
				return nil, err
			}
			if ref != nil {
				commit, err := repo.CommitObject(ref.Hash())
				if err != nil {
					return nil, err
				}
				ret[bn] = append(ret[bn], commit)
			}
		}
	}
	return ret, nil
}

func getBranchHashes(repo *git.Repository) (map[plumbing.Hash][]plumbing.ReferenceName, error) {
	ret := map[plumbing.Hash][]plumbing.ReferenceName{}
	brs, err := repo.Branches()
	if err != nil {
		return nil, err
	}
	if err := brs.ForEach(func(ref *plumbing.Reference) error {
		ret[ref.Hash()] = append(ret[ref.Hash()], ref.Name())
		return nil
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

type detector struct {
	repo *git.Repository
	// trunkCommits is a map from trunk branch name to the commits on the trunk branch.
	// Each trunk branch has at most two commits: the commit on the trunk branch itself and the
	// commit on the remote tracking branch.
	trunkCommits map[plumbing.ReferenceName][]*object.Commit
	// branches is a map from branch name to the hash of the commit on the branch.
	branches map[plumbing.Hash][]plumbing.ReferenceName
}

const iterStopErr = errors.Sentinel("stop")

func (d *detector) detectBranchTree(ref *plumbing.Reference) (*BranchPiece, error) {
	commit, err := d.repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}
	// Find the boundary of the search. We only need to search the commits until we hit the
	// trunk.
	trunkBases := map[plumbing.Hash][]plumbing.ReferenceName{}
	for trunkBranchName, commits := range d.trunkCommits {
		for _, trunkCommit := range commits {
			bases, err := trunkCommit.MergeBase(commit)
			if err != nil {
				return nil, err
			}
			if len(bases) > 1 {
				// The branch has a merge commit.
				return &BranchPiece{
					Name:                ref.Name(),
					ContainsMergeCommit: true,
				}, nil
			}
			if len(bases) == 1 {
				trunkBases[bases[0].Hash] = sliceutils.AppendIfNotContains(trunkBases[bases[0].Hash], trunkBranchName)
			}
		}
	}
	ret := &BranchPiece{
		Name: ref.Name(),
	}
	// Do a commit traversal. We can stop the traversal when we hit the trunk or if we find a
	// commit that has multiple parents.
	err = object.NewCommitPreorderIter(commit, nil, nil).ForEach(func(c *object.Commit) error {
		if trunkBranchNames, ok := trunkBases[c.Hash]; ok {
			if len(trunkBranchNames) > 1 {
				ret.PossibleParents = trunkBranchNames
				return iterStopErr
			}
			ret.Parent = trunkBranchNames[0]
			ret.ParentIsTrunk = true
			ret.ParentMergeBase = c.Hash
			return iterStopErr
		}
		if c.NumParents() > 1 {
			ret.ContainsMergeCommit = true
			return iterStopErr
		}
		if c.Hash != commit.Hash {
			if parents, ok := d.branches[c.Hash]; ok {
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

func mapToRemoteTrackingBranch(remoteConfig *config.RemoteConfig, refName plumbing.ReferenceName) *plumbing.ReferenceName {
	for _, fetch := range remoteConfig.Fetch {
		if fetch.Match(refName) {
			dst := fetch.Dst(refName)
			return &dst
		}
	}
	return nil
}
