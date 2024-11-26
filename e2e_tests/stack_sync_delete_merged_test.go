package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"
)

func TestStackSyncDeleteMerged(t *testing.T) {
	server := RunMockGitHubServer(t)
	defer server.Close()
	repo := gittest.NewTempRepoWithGitHubServer(t, server.URL)
	Chdir(t, repo.RepoDir)

	// To start, we create a simple two-stack where each stack has a single commit.
	// Our stack looks like:
	//     main:    X
	//     stack-1:  \ -> 1a -> 1b
	//     stack-2:              \ -> 2a -> 2b
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

	// Everything up to date now, so this should be a no-op.
	RequireAv(t, "sync")

	// We simulate the pull branches on the remote.
	repo.Git(t, "push", "origin", "stack-1:refs/pull/42/head")

	// We simulate a merge here so that our history looks like:
	//     main:    X --------------> 1S
	//     stack-1:  \ -> 1a -> 1b
	//     stack-2:              \ -> 2a -> 2b
	// where 1S is the squash-merge commit of 1b onto main. Note that since it's
	// a squash commit, 1S is not a *merge commit* in the Git definition.
	var squashCommit plumbing.Hash
	repo.WithCheckoutBranch(t, "refs/heads/main", func() {
		repo.Git(t, "merge", "--squash", "stack-1")
		// `git merge --squash` doesn't actually create the commit, so we have to
		// do that separately.
		repo.Git(t, "commit", "--no-edit")

		squashCommit = repo.GetCommitAtRef(t, plumbing.HEAD)

		repo.Git(t, "push", "origin", "main")
	})

	server.pulls = append(server.pulls, mockPR{
		ID:             "nodeid-42",
		Number:         42,
		State:          "MERGED",
		HeadRefName:    "stack-1",
		MergeCommitOID: squashCommit.String(),
	})

	// We shouldn't do this as part of an E2E test since it depends on internal
	// knowledge of the codebase, but :shrug:. We need to set the merge commit
	// manually since we can't actually communicate with the GitHub API as part
	// of this test.
	db := repo.OpenDB(t)
	tx := db.WriteTx()
	stack1Meta, _ := tx.Branch("stack-1")
	stack1Meta.PullRequest = &meta.PullRequest{ID: "nodeid-42", Number: 42}
	tx.SetBranch(stack1Meta)
	require.NoError(t, tx.Commit())

	repo.Git(t, "switch", "stack-1")
	RequireAv(t, "sync", "--prune=yes")

	require.Equal(t, 1,
		Cmd(t, "git", "show-ref", "refs/heads/stack-1").ExitCode,
		"stack-1 should be deleted after merge",
	)
	require.Equal(t, "refs/heads/main\n",
		repo.Git(t, "rev-parse", "--symbolic-full-name", "HEAD"),
		"HEAD should be on main after stack-1 is deleted",
	)
}

func TestStackSyncDeleteMerged_NoMain(t *testing.T) {
	server := RunMockGitHubServer(t)
	defer server.Close()
	repo := gittest.NewTempRepoWithGitHubServer(t, server.URL)
	Chdir(t, repo.RepoDir)

	RequireAv(t, "branch", "stack-1")
	repo.CommitFile(t, "my-file", "1a\n", gittest.WithMessage("Commit 1a"))
	repo.Git(t, "push", "origin", "stack-1:refs/pull/42/head")

	var squashCommit plumbing.Hash
	repo.WithCheckoutBranch(t, "refs/heads/main", func() {
		repo.Git(t, "merge", "--squash", "stack-1")
		// `git merge --squash` doesn't actually create the commit, so we have to
		// do that separately.
		repo.Git(t, "commit", "--no-edit")

		squashCommit = repo.GetCommitAtRef(t, plumbing.HEAD)

		repo.Git(t, "push", "origin", "main")
	})

	server.pulls = append(server.pulls, mockPR{
		ID:             "nodeid-42",
		Number:         42,
		State:          "MERGED",
		HeadRefName:    "stack-1",
		MergeCommitOID: squashCommit.String(),
	})

	// We shouldn't do this as part of an E2E test since it depends on internal
	// knowledge of the codebase, but :shrug:. We need to set the merge commit
	// manually since we can't actually communicate with the GitHub API as part
	// of this test.
	db := repo.OpenDB(t)
	tx := db.WriteTx()
	stack1Meta, _ := tx.Branch("stack-1")
	stack1Meta.PullRequest = &meta.PullRequest{ID: "nodeid-42", Number: 42}
	tx.SetBranch(stack1Meta)
	require.NoError(t, tx.Commit())

	repo.Git(t, "switch", "stack-1")
	repo.Git(t, "branch", "-D", "main")
	RequireAv(t, "sync", "--prune=yes")

	require.Equal(t, 1,
		Cmd(t, "git", "show-ref", "refs/heads/stack-1").ExitCode,
		"stack-1 should be deleted after merge",
	)
	require.Equal(t, "HEAD\n",
		repo.Git(t, "rev-parse", "--symbolic-full-name", "HEAD"),
		"HEAD should be on a detached HEAD after stack-1 is deleted",
	)
	require.Equal(t,
		repo.Git(t, "rev-parse", "HEAD"),
		repo.Git(t, "rev-parse", "origin/main"),
		"HEAD should be on origin/main",
	)
}
