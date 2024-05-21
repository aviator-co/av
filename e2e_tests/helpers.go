package e2e_tests

import (
	"os"
	"testing"
	"time"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"
)

func RequireCurrentBranchName(t *testing.T, repo *gittest.GitTestRepo, name plumbing.ReferenceName) {
	ref, err := repo.GoGit.Reference(plumbing.HEAD, false)
	require.NoError(t, err, "failed to determine current branch name")
	require.Equal(t, plumbing.SymbolicReference, ref.Type(), "expected HEAD to be a symbolic reference")
	require.Equal(
		t,
		name,
		ref.Target(),
		"expected current branch to be %q, got %q",
		name,
		ref.Target(),
	)
}

func GetFetchHeadTimestamp(t *testing.T, repo *gittest.GitTestRepo) time.Time {
	fileInfo, err := os.Stat(repo.RepoDir + "/.git/FETCH_HEAD")
	require.NoError(t, err, "failed to stat .git/FETCH_HEAD")
	return fileInfo.ModTime()
}

func GetStoredParentBranchState(t *testing.T, repo *gittest.GitTestRepo, name string) meta.BranchState {
	// We shouldn't do this as part of an E2E test, but it's hard to ensure otherwise.
	db := repo.OpenDB(t)
	br, _ := db.ReadTx().Branch(name)
	return br.Parent
}
