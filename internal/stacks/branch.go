package stacks

import "github.com/aviator-co/av/internal/git"

type Branch struct {
	metadata *branchMetadata
}

// GetBranch loads information about the branch from the git repository.
// Returns nil if given branch does not exist or is not a stacked branch.
func GetBranch(repo *git.Repo, branchName string) (*Branch, error) {
	meta := readStackMetadata(repo, branchName)
	if meta == nil {
		return nil, nil
	}
	return &Branch{meta}, nil
}

func (b *Branch) ParentBranchName() string {
	return b.metadata.Parent
}
