package git_test

import (
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
)

func TestWriteStateFile(t *testing.T) {
	type Test struct {
		Message string
	}

	tmpRepo := gittest.NewTempRepo(t)
	repo, err := git.OpenRepo(tmpRepo.RepoDir, tmpRepo.GitDir)
	require.NoError(t, err)

	err = repo.WriteStateFile(git.StateFileKindSync, &Test{Message: "test write kind sync"})
	require.NoError(t, err)

	// Already state file exists, thus it should return an error
	err = repo.WriteStateFile(git.StateFileKindSync, &Test{Message: "test write kind sync 2"})
	require.Error(t, err)

	// clean up
	err = repo.WriteStateFile(git.StateFileKindSync, nil)
	require.NoError(t, err)

	// Write again
	err = repo.WriteStateFile(git.StateFileKindSync, &Test{Message: "test write kind sync 2"})
	require.NoError(t, err)
}
