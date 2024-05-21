package e2e_tests

import (
	"os"
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
)

func TestStackBranchCommit(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	t.Run("FlagAll", func(t *testing.T) {
		require.NoError(t, os.WriteFile("myfile.txt", []byte("hello\n"), 0644))
		RequireAv(t, "stack", "branch-commit", "--all", "-m", "branch one")
		require.True(t, repo.IsWorkdirClean(t))
	})

	t.Run("FlagAllModified", func(t *testing.T) {
		require.NoError(t, os.WriteFile("myfile.txt", []byte("hello\nworld\n"), 0644))
		require.NoError(t, os.WriteFile("yourfile.txt", []byte("bonjour\n"), 0644))
		RequireAv(t, "stack", "branch-commit", "--all-modified", "-m", "branch two")

		require.False(
			t,
			repo.IsWorkdirClean(t),
			"workdir should not be clean since yourfile.txt should not be committed",
		)

		diffout := repo.Git(t, "diff", "myfile.txt")
		require.Empty(t, diffout, "myfile.txt should be committed and not have a diff")

		lsout := repo.Git(t, "ls-files", "yourfile.txt")
		require.Empty(t, lsout, "yourfile.txt should not be committed")
	})
}
