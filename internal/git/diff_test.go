package git_test

import (
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"
)

func TestRepoDiffAmbiguousPathName(t *testing.T) {
	repo := gittest.NewTempRepo(t)

	repo.CreateRef(t, plumbing.NewBranchReferenceName("foo"))
	repo.CheckoutBranch(t, plumbing.NewBranchReferenceName("foo"))

	repo.CommitFile(t, "foo", "foo")
	diff, err := repo.AsAvGitRepo().Diff(&git.DiffOpts{
		Quiet:      true,
		Specifiers: []string{"main", "foo"},
	})
	require.NoError(t, err, "repo.Diff should not error given an ambiguous branch/path name")
	require.False(
		t,
		diff.Empty,
		"diff between branches with different trees should return non-empty",
	)
}
