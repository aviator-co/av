package git_test

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
)

func TestOrigin(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	_, err := repo.Git("remote", "set-url", "origin", "https://github.com/aviator-co/av.git")
	require.NoError(t, err)
	origin, err := repo.Origin()
	require.NoError(t, err)
	require.Equal(t, "aviator-co/av", origin.RepoSlug)

	_, err = repo.Git("remote", "set-url", "origin", "git@github.com:aviator-co/av.git")
	require.NoError(t, err)
	origin, err = repo.Origin()
	require.NoError(t, err)
	require.Equal(t, "aviator-co/av", origin.RepoSlug)
}
