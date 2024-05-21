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

	c1 := repo.CommitFile(t, "file", "first commit\n")
	c2 := repo.CommitFile(t, "file", "first commit\nsecond commit\n")

	// Switch back to c1 and test that we can cherry-pick c2 on top of it
	repo.CheckoutCommit(t, c1)
	require.NoError(t, repo.AsAvGitRepo().CherryPick(git.CherryPick{Commits: []string{c2.String()}}))
	contents, err := os.ReadFile(filepath.Join(repo.RepoDir, "file"))
	require.NoError(t, err)
	assert.Equal(t, "first commit\nsecond commit\n", string(contents))

	// Switch back to c1 and check that we can fast-forward to c2
	repo.CheckoutCommit(t, c1)
	require.NoError(t, repo.AsAvGitRepo().CherryPick(git.CherryPick{Commits: []string{c2.String()}, FastForward: true}))
	_, err = os.ReadFile(filepath.Join(repo.RepoDir, "file"))
	require.NoError(t, err)

	// We're back to c2, so trying to cherry-pick c1 should fail.
	err = repo.AsAvGitRepo().CherryPick(git.CherryPick{Commits: []string{c1.String()}})
	conflictErr, ok := errutils.As[git.ErrCherryPickConflict](err)
	require.True(t, ok, "expected cherry-pick conflict")
	assert.Equal(t, c1.String(), conflictErr.ConflictingCommit)
}
