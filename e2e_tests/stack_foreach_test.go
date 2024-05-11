package e2e_tests

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aviator-co/av/internal/git/gittest"
)

func TestStackForEach(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	// Create a stack of three branches
	RequireAv(t, "stack", "branch", "stack-1")
	repo.CommitFile(t, "my-file", "1a\n", gittest.WithMessage("Commit 1a"))
	RequireAv(t, "stack", "branch", "stack-2")
	repo.CommitFile(t, "my-file", "2a\n", gittest.WithMessage("Commit 2a"))
	RequireAv(t, "stack", "branch", "stack-3")
	repo.CommitFile(t, "my-file", "3a\n", gittest.WithMessage("Commit 3a"))

	out := RequireAv(t,
		"stack", "for-each", "--",
		"git", "show", "--format=%s", "--quiet", "HEAD", "--",
	)
	require.Equal(t, "Commit 1a\nCommit 2a\nCommit 3a\n", out.Stdout)
}
