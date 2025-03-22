package git_test

import (
	"testing"

	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
)

func TestOrigin(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	repo.Git(t, "remote", "set-url", "origin", "https://github.com/aviator-co/av.git")
	origin, err := repo.AsAvGitRepo().Origin()
	require.NoError(t, err)
	require.Equal(t, "aviator-co/av", origin.RepoSlug)

	repo.Git(t, "remote", "set-url", "origin", "git@github.com:aviator-co/av.git")
	require.NoError(t, err)
	origin, err = repo.AsAvGitRepo().Origin()
	require.NoError(t, err)
	require.Equal(t, "aviator-co/av", origin.RepoSlug)
}

func TestTrunkBranches(t *testing.T) {
	repo := gittest.NewTempRepo(t)

	branches, err := repo.AsAvGitRepo().TrunkBranches()
	require.NoError(t, err)
	require.Equal(t, branches, []string{"main"})

	// add some branches to AdditionalTrunkBranches
	config.Av.AdditionalTrunkBranches = []string{"develop", "staging"}
	branches, err = repo.AsAvGitRepo().TrunkBranches()
	require.NoError(t, err)
	require.Equal(t, branches, []string{"main", "develop", "staging"})
}

func TestGetRemoteName(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	require.Equal(t, repo.AsAvGitRepo().GetRemoteName(), git.DEFAULT_REMOTE_NAME)

	config.Av.Remote = "new-remote"
	require.Equal(t, repo.AsAvGitRepo().GetRemoteName(), "new-remote")
}
