package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStackSyncMergedParent(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.Dir())

	// To start, we create a simple two-stack where each stack has a single commit.
	// Our stack looks like:
	//     stack-1: main -> 1a -> 2b
	//     stack-2:                \ -> 2a -> 2b
	//     stack-3:                             \ -> 3a -> 3b
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

	// Everything up to date now, so this should be a no-op.
	require.Equal(t, 0, Av(t, "stack", "sync", "--no-fetch", "--no-push").ExitCode)

	// We simulate a merge here so that our history looks like:
	//     main:    X
	//     stack-1:  \ -> 1a -> 2b           / -> 1S
	//     stack-2:              \ -> 2a -> 2b
	// where 2S is the squash-merge commit of 2b onto stack-1. Note that since it's
	// a squash commit, 2S is not a *merge commit* in the Git definition.
	var squashCommit string
	gittest.WithCheckoutBranch(t, repo, "stack-1", func() {
		oldHead, err := repo.RevParse(&git.RevParse{Rev: "HEAD"})
		require.NoError(t, err, "failed to get HEAD")

		RequireCmd(t, "git", "merge", "--squash", "stack-2")
		// `git merge --squash` doesn't actually create the commit, so we have to
		// do that separately.
		RequireCmd(t, "git", "commit", "--no-edit")
		squashCommit, err = repo.RevParse(&git.RevParse{Rev: "HEAD"})
		require.NoError(t, err, "failed to get squash commit")
		require.NotEqual(
			t,
			oldHead,
			squashCommit,
			"squash commit should be different from old HEAD",
		)
	})

	// We shouldn't do this as part of an E2E test since it depends on internal
	// knowledge of the codebase, but :shrug:. We need to set the merge commit
	// manually since we can't actually communicate with the GitHub API as part
	// of this test.
	db, err := jsonfiledb.OpenRepo(repo)
	require.NoError(t, err, "failed to open repo db")
	tx := db.WriteTx()
	stack2Meta, _ := tx.Branch("stack-2")
	stack2Meta.MergeCommit = squashCommit
	tx.SetBranch(stack2Meta)
	require.NoError(t, tx.Commit())

	require.Equal(t, 0,
		Cmd(t, "git", "merge-base", "--is-ancestor", "stack-2", "stack-3").ExitCode,
		"HEAD of stack-1 should be an ancestor of HEAD of stack-2 before running sync",
	)
	require.NotEqual(t, 0,
		Cmd(t, "git", "merge-base", "--is-ancestor", squashCommit, "stack-3").ExitCode,
		"squash commit of stack-1 should not be an ancestor of HEAD of stack-1 before running sync",
	)

	RequireAv(t, "stack", "sync", "--no-fetch", "--no-push")

	assert.Equal(t, 0,
		Cmd(t, "git", "merge-base", "--is-ancestor", squashCommit, "stack-3").ExitCode,
		"squash commit of stack-2 should be an ancestor of HEAD of stack-3 after running sync",
	)
	assert.Equal(t,
		meta.BranchState{
			Name: "stack-1",
			Head: squashCommit,
		},
		GetStoredParentBranchState(t, repo, "stack-3"),
		"stack-3 should be re-rooted onto stack-1",
	)
}
