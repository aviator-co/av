package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
)

func TestStackSyncDeleteParent(t *testing.T) {
	server := RunMockGitHubServer(t)
	defer server.Close()
	repo := gittest.NewTempRepoWithGitHubServer(t, server.URL)
	Chdir(t, repo.RepoDir)

	// To start, we create a simple two-stack where each stack has a single commit.
	// Our stack looks like:
	//     stack-1: main -> 1a -> 2b
	//     stack-2:                \ -> 2a -> 2b
	//     stack-3:	                           \ -> 3a -> 3b
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

	// Everything up to date now, so this should be a no-op.
	RequireAv(t, "stack", "sync")

	// We simulate the stack-2 is deleted and submerged into stack-1
	//     main:    X
	//     stack-1:  \ -> 1a -> 1b -> 2a -> 2b
	var newStack1Head plumbing.Hash
	repo.WithCheckoutBranch(t, "refs/heads/stack-1", func() {
		repo.Git(t, "merge", "--ff-only", "stack-2")
		repo.Git(t, "branch", "-D", "stack-2")

		newStack1Head = repo.GetCommitAtRef(t, plumbing.HEAD)
	})
	RequireAv(t, "stack", "tidy")
	RequireAv(t, "stack", "sync")

	// stack-1 should be an ancestor of stack-3 after running sync
	repo.Git(t, "merge-base", "--is-ancestor", newStack1Head.String(), "stack-3")
	assert.Equal(t,
		meta.BranchState{
			Name: "stack-1",
			Head: newStack1Head.String(),
		},
		GetStoredParentBranchState(t, repo, "stack-3"),
		"stack-3 should be re-rooted onto stack-1",
	)
}
