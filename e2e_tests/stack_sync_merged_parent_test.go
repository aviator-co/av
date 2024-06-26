package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStackSyncMergedParent(t *testing.T) {
	server := RunMockGitHubServer(t)
	defer server.Close()
	repo := gittest.NewTempRepoWithGitHubServer(t, server.URL)
	Chdir(t, repo.RepoDir)

	// To start, we create a simple two-stack where each stack has a single commit.
	// Our stack looks like:
	//     stack-1: main -> 1a -> 2b
	//     stack-2:                \ -> 2a -> 2b
	//     stack-3:                             \ -> 3a -> 3b
	RequireAv(t, "stack", "branch", "stack-1")
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

	// We simulate a merge here so that our history looks like:
	//     main:    X
	//     stack-1:  \ -> 1a -> 2b           / -> 1S
	//     stack-2:              \ -> 2a -> 2b
	// where 2S is the squash-merge commit of 2b onto stack-1. Note that since it's
	// a squash commit, 2S is not a *merge commit* in the Git definition.
	var squashCommit plumbing.Hash
	repo.WithCheckoutBranch(t, "refs/heads/main", func() {
		oldHead := repo.GetCommitAtRef(t, plumbing.HEAD)

		repo.Git(t, "merge", "--squash", "stack-2")
		// `git merge --squash` doesn't actually create the commit, so we have to
		// do that separately.
		repo.Git(t, "commit", "--no-edit")
		squashCommit = repo.GetCommitAtRef(t, plumbing.HEAD)

		require.NotEqual(
			t,
			oldHead,
			squashCommit,
			"squash commit should be different from old HEAD",
		)
		repo.Git(t, "push", "origin", "main")
	})

	// We shouldn't do this as part of an E2E test since it depends on internal
	// knowledge of the codebase, but :shrug:. We need to set the merge commit
	// manually since we can't actually communicate with the GitHub API as part
	// of this test.
	db := repo.OpenDB(t)
	tx := db.WriteTx()
	stack2Meta, _ := tx.Branch("stack-2")
	stack2Meta.MergeCommit = squashCommit.String()
	tx.SetBranch(stack2Meta)
	require.NoError(t, tx.Commit())

	// HEAD of stack-1 should be an ancestor of HEAD of stack-2 before running sync
	repo.Git(t, "merge-base", "--is-ancestor", "stack-2", "stack-3")
	require.NotEqual(t, 0,
		Cmd(t, "git", "merge-base", "--is-ancestor", squashCommit.String(), "stack-3").ExitCode,
		"squash commit of stack-1 should not be an ancestor of HEAD of stack-1 before running sync",
	)

	RequireAv(t, "stack", "sync")

	assert.Equal(t,
		meta.BranchState{
			Name:  "main",
			Trunk: true,
		},
		GetStoredParentBranchState(t, repo, "stack-3"),
		"stack-3 should be re-rooted onto main",
	)
}
