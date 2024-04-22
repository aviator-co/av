package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
)

func TestStackOrphan(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.Dir())

	// To start, we create a simple two-stack where each stack has a single commit.
	// Our stack looks like:
	//     stack-1: main -> 1a -> 1b
	//     stack-2:                \ -> 2a -> 2b
	//     stack-3:	                           \ -> 3a -> 3b
	// Then stack-2 (and child branch stack-3) will be orphaned

	// Setup initial state
	require.Equal(t, 0, Cmd(t, "git", "checkout", "-b", "stack-1").ExitCode)
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n"), gittest.WithMessage("Commit 1a"))
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n1b\n"), gittest.WithMessage("Commit 1b"))
	RequireAv(t, "stack", "branch", "stack-2")
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n1b\n2a\n"), gittest.WithMessage("Commit 2a"))
	gittest.CommitFile(
		t,
		repo,
		"my-file",
		[]byte("1a\n1b\n2a\n2b\n"),
		gittest.WithMessage("Commit 2b"),
	)
	RequireAv(t, "stack", "branch", "stack-3")
	gittest.CommitFile(
		t,
		repo,
		"my-file",
		[]byte("1a\n1b\n2a\n2b\n3a\n"),
		gittest.WithMessage("Commit 3a"),
	)
	gittest.CommitFile(
		t,
		repo,
		"my-file",
		[]byte("1a\n1b\n2a\n2b\n3a\n3b\n"),
		gittest.WithMessage("Commit 3b"),
	)

	RequireAv(t, "stack", "prev")
	tree := Av(t, "stack", "tree")
	require.Contains(t, tree.Stdout, "stack-2")
	require.Contains(t, tree.Stdout, "stack-3")

	RequireAv(t, "stack", "orphan")

	tree = Av(t, "stack", "tree")
	require.NotContains(t, tree.Stdout, "stack-2")
	require.NotContains(t, tree.Stdout, "stack-3")

}
