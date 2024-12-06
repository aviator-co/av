package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
)

func TestOrphan(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	// To start, we create a simple two-stack where each stack has a single commit.
	// Our stack looks like:
	//     stack-1: main -> 1a -> 1b
	//     stack-2:                \ -> 2a -> 2b
	//     stack-3:	                           \ -> 3a -> 3b
	// Then stack-2 (and child branch stack-3) will be orphaned

	// Setup initial state
	RequireAv(t, "branch", "stack-1")
	repo.CommitFile(t, "my-file", "1a\n", gittest.WithMessage("Commit 1a"))
	repo.CommitFile(t, "my-file", "1a\n1b\n", gittest.WithMessage("Commit 1b"))
	RequireAv(t, "branch", "stack-2")
	repo.CommitFile(t, "my-file", "1a\n1b\n2a\n", gittest.WithMessage("Commit 2a"))
	repo.CommitFile(
		t,
		"my-file",
		"1a\n1b\n2a\n2b\n",
		gittest.WithMessage("Commit 2b"),
	)
	RequireAv(t, "branch", "stack-3")
	repo.CommitFile(
		t,
		"my-file",
		"1a\n1b\n2a\n2b\n3a\n",
		gittest.WithMessage("Commit 3a"),
	)
	repo.CommitFile(
		t,
		"my-file",
		"1a\n1b\n2a\n2b\n3a\n3b\n",
		gittest.WithMessage("Commit 3b"),
	)

	RequireAv(t, "prev")
	tree := RequireAv(t, "tree")
	require.Contains(t, tree.Stdout, "stack-2")
	require.Contains(t, tree.Stdout, "stack-3")

	RequireAv(t, "orphan")

	tree = RequireAv(t, "tree")
	require.NotContains(t, tree.Stdout, "stack-2")
	require.NotContains(t, tree.Stdout, "stack-3")
}
