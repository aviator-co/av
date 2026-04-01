package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
)

func TestSyncAllWithDeletedBranch(t *testing.T) {
	server := RunMockGitHubServer(t)
	defer server.Close()
	repo := gittest.NewTempRepoWithGitHubServer(t, server.URL)
	Chdir(t, repo.RepoDir)

	// Create two independent stacks off main.
	repo.Git(t, "switch", "main")
	RequireAv(t, "branch", "stack-1")
	repo.CommitFile(t, "my-file", "1a\n", gittest.WithMessage("Commit 1a"))

	repo.Git(t, "switch", "main")
	RequireAv(t, "branch", "stack-2")
	repo.CommitFile(t, "my-file-2", "2a\n", gittest.WithMessage("Commit 2a"))

	// Delete stack-1's git ref without tidying av metadata.
	// This simulates a branch deleted externally (e.g., on GitHub) but not yet
	// cleaned up in av's internal database.
	repo.Git(t, "switch", "stack-2")
	repo.Git(t, "branch", "-D", "stack-1")

	// Push a new commit to main so there's something to sync.
	repo.WithCheckoutBranch(t, "refs/heads/main", func() {
		repo.CommitFile(t, "other-file", "X2\n", gittest.WithMessage("Commit X2"))
		repo.Git(t, "push", "origin", "main")
	})

	// This should succeed, skipping the deleted branch rather than failing
	// with "error: git merge-base: exit status 128".
	RequireAv(t, "sync", "--all", "--rebase-to-trunk")

	// stack-2 should have been synced onto the latest main.
	repo.Git(t, "merge-base", "--is-ancestor", "main", "stack-2")
}
