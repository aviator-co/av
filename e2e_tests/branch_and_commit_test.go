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
		require.NoError(t, os.WriteFile("myfile.txt", []byte("hello\n"), 0o644))
		RequireAv(t, "commit", "-b", "--all-changes", "-m", "branch one")
		require.True(t, repo.IsWorkdirClean(t))
	})

	t.Run("FlagAllModified", func(t *testing.T) {
		require.NoError(t, os.WriteFile("myfile.txt", []byte("hello\nworld\n"), 0o644))
		require.NoError(t, os.WriteFile("yourfile.txt", []byte("bonjour\n"), 0o644))
		RequireAv(t, "commit", "-b", "--all", "-m", "branch two")

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
