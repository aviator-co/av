package git_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/utils/errutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepo_CherryPick(t *testing.T) {
	repo := gittest.NewTempRepo(t)

	c1 := gittest.CommitFile(t, repo, "file", []byte("first commit\n"))
	c2 := gittest.CommitFile(t, repo, "file", []byte("first commit\nsecond commit\n"))

	// Switch back to c1 and test that we can cherry-pick c2 on top of it
	if _, err := repo.CheckoutBranch(&git.CheckoutBranch{Name: c1}); err != nil {
		t.Fatal(err)
	}
	require.NoError(t, repo.CherryPick(git.CherryPick{Commits: []string{c2}}))
	contents, err := os.ReadFile(filepath.Join(repo.Dir(), "file"))
	require.NoError(t, err)
	assert.Equal(t, "first commit\nsecond commit\n", string(contents))

	// Switch back to c1 and check that we can fast-forward to c2
	if _, err := repo.CheckoutBranch(&git.CheckoutBranch{Name: c1}); err != nil {
		t.Fatal(err)
	}
	require.NoError(t, repo.CherryPick(git.CherryPick{Commits: []string{c2}, FastForward: true}))
	contents, err = os.ReadFile(filepath.Join(repo.Dir(), "file"))
	require.NoError(t, err)

	// We're back to c2, so trying to cherry-pick c1 should fail.
	err = repo.CherryPick(git.CherryPick{Commits: []string{c1}})
	conflictErr, ok := errutils.As[git.ErrCherryPickConflict](err)
	require.True(t, ok, "expected cherry-pick conflict")
	assert.Equal(t, c1, conflictErr.ConflictingCommit)
}
