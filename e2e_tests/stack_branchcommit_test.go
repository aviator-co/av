package e2e_tests

import (
	"os"
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
)

func TestStackBranchCommit(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.Dir())

	t.Run("FlagAll", func(t *testing.T) {
		require.NoError(t, os.WriteFile("myfile.txt", []byte("hello\n"), 0644))
		RequireAv(t, "stack", "branch-commit", "--all", "-m", "branch one")
		clean, err := repo.CheckCleanWorkdir()
		require.NoError(t, err)
		require.True(t, clean)
	})

	t.Run("FlagAllModified", func(t *testing.T) {
		require.NoError(t, os.WriteFile("myfile.txt", []byte("hello\nworld\n"), 0644))
		require.NoError(t, os.WriteFile("yourfile.txt", []byte("bonjour\n"), 0644))
		RequireAv(t, "stack", "branch-commit", "--all-modified", "-m", "branch two")

		clean, err := repo.CheckCleanWorkdir()
		require.NoError(t, err)
		require.False(
			t,
			clean,
			"workdir should not be clean since yourfile.txt should not be committed",
		)

		diff, err := repo.Diff(&git.DiffOpts{
			Quiet: true,
			Paths: []string{"myfile.txt"},
		})
		require.NoError(t, err)
		require.True(t, diff.Empty, "myfile.txt should be committed and not have a diff")

		lsout, err := repo.Git("ls-files", "yourfile.txt")
		require.NoError(t, err)
		require.Empty(t, lsout, "yourfile.txt should not be committed")
	})
}
