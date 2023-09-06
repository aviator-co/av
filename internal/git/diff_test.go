package git_test

import (
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
)

func TestRepoDiffAmbiguousPathName(t *testing.T) {
	repo := gittest.NewTempRepo(t)

	_, err := repo.CheckoutBranch(&git.CheckoutBranch{Name: "foo", NewBranch: true})
	require.NoError(t, err, "repo.CheckoutBranch should not error given a valid branch name")

	gittest.CommitFile(t, repo, "foo", []byte("foo"))
	diff, err := repo.Diff(&git.DiffOpts{
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
