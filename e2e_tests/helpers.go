package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/stretchr/testify/require"
)

func RequireCurrentBranchName(t *testing.T, repo *git.Repo, name string) {
	currentBranch, err := repo.CurrentBranchName()
	require.NoError(t, err, "failed to determine current branch name")
	require.Equal(t, name, currentBranch, "expected current branch to be %q, got %q", name, currentBranch)
}
