package git_test

import (
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRemote(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	remote, err := repo.DefaultRemote()

	require.NoError(t, err)
	require.Equal(t, "origin", remote.Label)
	require.Equal(t, "aviator-co/nonexistent-repo", remote.RepoSlug)

	// can be accessed via remote label
	remote, err = repo.Remote("origin")

	require.NoError(t, err)
	require.Equal(t, "origin", remote.Label)
	require.Equal(t, "aviator-co/nonexistent-repo", remote.RepoSlug)

	// works with set-url changes
	_, err = repo.Git("remote", "set-url", remote.Label, "https://github.com/aviator-co/av.git")
	require.NoError(t, err)

	remote, err = repo.DefaultRemote()
	require.NoError(t, err)
	require.Equal(t, "origin", remote.Label)
	require.Equal(t, "aviator-co/av", remote.RepoSlug)

	_, err = repo.Git("remote", "set-url", remote.Label, "git@github.com:aviator-co/av.git")
	require.NoError(t, err)

	remote, err = repo.DefaultRemote()
	require.NoError(t, err)
	require.Equal(t, "origin", remote.Label)
	require.Equal(t, "aviator-co/av", remote.RepoSlug)

	// add additional remote
	_, err = repo.Git("remote", "add", "test", "https://github.com/aviator-co/test-repo.git")
	require.NoError(t, err)

	// default remote has not changed
	remote, err = repo.DefaultRemote()
	require.NoError(t, err)
	require.Equal(t, "origin", remote.Label)
	require.Equal(t, "aviator-co/av", remote.RepoSlug)

	// get remote by label
	remote, err = repo.Remote("test")
	require.NoError(t, err)
	require.Equal(t, "test", remote.Label)
	require.Equal(t, "aviator-co/test-repo", remote.RepoSlug)

	// remove origin remote
	_, err = repo.Git("remote", "remove", "origin")
	require.NoError(t, err)

	// test is now default
	remote, err = repo.DefaultRemote()

	require.NoError(t, err)
	require.Equal(t, "test", remote.Label)
	require.Equal(t, "aviator-co/test-repo", remote.RepoSlug)

	// request for origin causes error
	_, err = repo.Remote("origin")
	require.ErrorContains(t, err, "no remote config found for origin")

	// remove test remote
	_, err = repo.Git("remote", "remove", "test")
	require.NoError(t, err)

	// request for default causes error
	_, err = repo.DefaultRemote()
	require.ErrorContains(t, err, "no remote config found")
}
