package e2e_tests

import (
	"os"
	"testing"
	"time"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/stretchr/testify/require"
)

func RequireCurrentBranchName(t *testing.T, repo *git.Repo, name string) {
	currentBranch, err := repo.CurrentBranchName()
	require.NoError(t, err, "failed to determine current branch name")
	require.Equal(
		t,
		name,
		currentBranch,
		"expected current branch to be %q, got %q",
		name,
		currentBranch,
	)
}

func GetFetchHeadTimestamp(t *testing.T, repo *git.Repo) time.Time {
	fileInfo, err := os.Stat(repo.Dir() + "/.git/FETCH_HEAD")
	require.NoError(t, err, "failed to stat .git/FETCH_HEAD")
	return fileInfo.ModTime()
}

func GetStoredParentBranchState(t *testing.T, repo *git.Repo, name string) meta.BranchState {
	// We shouldn't do this as part of an E2E test, but it's hard to ensure otherwise.
	db, err := jsonfiledb.OpenRepo(repo)
	require.NoError(t, err, "failed to open repo db")
	br, _ := db.ReadTx().Branch(name)
	return br.Parent
}
