package stacks

import (
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
)

// BranchMetadata is metadata about a given branch.
// TODO:
//     Delete this type alias and just use meta.Branch.
type BranchMetadata = meta.Branch

// GetMetadata loads information about the branch from the git repository.
// Returns nil if given branch does not exist or is not a stacked branch.
func GetMetadata(repo *git.Repo, branchName string) *BranchMetadata {
	branch, ok := meta.GetBranch(repo, branchName)
	if !ok {
		return nil
	}
	return &branch
}

func writeMetadata(repo *git.Repo, s *BranchMetadata) error {
	return meta.WriteBranch(repo, *s)
}
