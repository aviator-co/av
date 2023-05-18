package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/meta"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/stretchr/testify/require"
)

func TestStackSyncDeleteMerged(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.Dir())

	// To start, we create a simple two-stack where each stack has a single commit.
	// Our stack looks like:
	//     main:    X
	//     stack-1:  \ -> 1a -> 1b
	//     stack-2:              \ -> 2a -> 2b
	require.Equal(t, 0, Cmd(t, "git", "checkout", "-b", "stack-1").ExitCode)
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n"), gittest.WithMessage("Commit 1a"))
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n1b\n"), gittest.WithMessage("Commit 1b"))
	RequireAv(t, "stack", "branch", "stack-2")
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n1b\n2a\n"), gittest.WithMessage("Commit 2a"))
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n1b\n2a\n2b\n"), gittest.WithMessage("Commit 2b"))

	// Everything up to date now, so this should be a no-op.
	require.Equal(t, 0, Av(t, "stack", "sync", "--no-fetch", "--no-push").ExitCode)

	// We simulate the pull branches on the remote.
	RequireCmd(t, "git", "push", "origin", "stack-1:refs/pull/42/head")

	// We simulate a merge here so that our history looks like:
	//     main:    X --------------> 1S
	//     stack-1:  \ -> 1a -> 1b
	//     stack-2:              \ -> 2a -> 2b
	// where 1S is the squash-merge commit of 1b onto main. Note that since it's
	// a squash commit, 1S is not a *merge commit* in the Git definition.
	var squashCommit string
	gittest.WithCheckoutBranch(t, repo, "main", func() {
		RequireCmd(t, "git", "merge", "--squash", "stack-1")
		// `git merge --squash` doesn't actually create the commit, so we have to
		// do that separately.
		RequireCmd(t, "git", "commit", "--no-edit")

		var err error
		squashCommit, err = repo.RevParse(&git.RevParse{Rev: "HEAD"})
		require.NoError(t, err, "failed to get squash commit")

		RequireCmd(t, "git", "push", "origin", "main")
	})

	// We shouldn't do this as part of an E2E test since it depends on internal
	// knowledge of the codebase, but :shrug:. We need to set the merge commit
	// manually since we can't actually communicate with the GitHub API as part
	// of this test.
	db, err := jsonfiledb.OpenRepo(repo)
	require.NoError(t, err, "failed to open repo db")
	tx := db.WriteTx()
	stack1Meta, _ := tx.Branch("stack-1")
	stack1Meta.MergeCommit = squashCommit
	stack1Meta.PullRequest = &meta.PullRequest{Number: 42}
	tx.SetBranch(stack1Meta)
	require.NoError(t, tx.Commit())

	RequireAv(t, "stack", "sync", "--no-fetch", "--no-push", "--trunk", "--prune")

	require.Equal(t, 1,
		Cmd(t, "git", "show-ref", "refs/heads/stack-1").ExitCode,
		"stack-1 should be deleted after merge",
	)
}
