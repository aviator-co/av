package git_test

import (
	"testing"

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
