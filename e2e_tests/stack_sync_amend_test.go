package e2e_tests

import (
	"os"
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
)

func TestSyncAfterAmendingCommit(t *testing.T) {
	server := RunMockGitHubServer(t)
	defer server.Close()
	repo := gittest.NewTempRepoWithGitHubServer(t, server.URL)
	Chdir(t, repo.RepoDir)

	// Create a three stack...
	repo.Git(t, "checkout", "-b", "stack-1")
	repo.CommitFile(t, "my-file", "1a\n", gittest.WithMessage("Commit 1a"))
	repo.CommitFile(t, "my-file", "1a\n1b\n", gittest.WithMessage("Commit 1b"))
	RequireAv(t, "stack", "branch", "stack-2")
	repo.CommitFile(t, "my-file", "1a\n1b\n2a\n", gittest.WithMessage("Commit 2a"))
	repo.CommitFile(
		t,
		"my-file",
		"1a\n1b\n2a\n2b\n",
		gittest.WithMessage("Commit 2b"),
	)
	RequireAv(t, "stack", "branch", "stack-3")
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

	// Now we amend commit 1b and make sure the sync after succeeds
	repo.CheckoutBranch(t, "refs/heads/stack-1")
	repo.CommitFile(t, "my-file", "1a\n1c\n1b\n", gittest.WithAmend())
	RequireAv(t, "stack", "sync")
	repo.CheckoutBranch(t, "refs/heads/stack-3")
	contents, err := os.ReadFile("my-file")
	require.NoError(t, err)
	require.Equal(t, "1a\n1c\n1b\n2a\n2b\n3a\n3b\n", string(contents))

	// Now we amend commit 2a and make sure the sync succeeds
	repo.CheckoutBranch(t, "refs/heads/stack-2")
}
