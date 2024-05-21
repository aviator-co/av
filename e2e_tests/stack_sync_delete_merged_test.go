package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"
)

func TestStackSyncDeleteMerged(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	// To start, we create a simple two-stack where each stack has a single commit.
	// Our stack looks like:
	//     main:    X
	//     stack-1:  \ -> 1a -> 1b
	//     stack-2:              \ -> 2a -> 2b
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

	// Everything up to date now, so this should be a no-op.
	RequireAv(t, "stack", "sync", "--no-fetch", "--no-push")

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

	// We shouldn't do this as part of an E2E test since it depends on internal
	// knowledge of the codebase, but :shrug:. We need to set the merge commit
	// manually since we can't actually communicate with the GitHub API as part
	// of this test.
	db := repo.OpenDB(t)
	tx := db.WriteTx()
	stack1Meta, _ := tx.Branch("stack-1")
	stack1Meta.MergeCommit = squashCommit.String()
	stack1Meta.PullRequest = &meta.PullRequest{Number: 42}
	tx.SetBranch(stack1Meta)
	require.NoError(t, tx.Commit())

	RequireAv(t, "stack", "sync", "--no-fetch", "--no-push", "--trunk", "--prune")

	require.Equal(t, 1,
		Cmd(t, "git", "show-ref", "refs/heads/stack-1").ExitCode,
		"stack-1 should be deleted after merge",
	)
}
