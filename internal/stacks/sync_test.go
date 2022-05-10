package stacks_test

import (
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/stacks"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"path"
	"testing"
)

func TestSyncBranch_NoConflicts(t *testing.T) {
	for _, strategy := range []stacks.SyncStrategy{
		stacks.StrategyMergeCommit,
		stacks.StrategyRebase,
	} {
		repo := gittest.NewTempRepo(t)

		_, err := repo.CheckoutBranch(&git.CheckoutBranch{Name: "stack-1", NewBranch: true})
		require.NoError(t, err)
		gittest.CommitFile(t, repo, "file1.txt", []byte("file1"))

		_, err = repo.CheckoutBranch(&git.CheckoutBranch{Name: "stack-2", NewBranch: true})
		require.NoError(t, err)
		gittest.CommitFile(t, repo, "file2.txt", []byte("file2"))

		res, err := stacks.SyncBranch(repo, &stacks.SyncBranchOpts{
			Parent:   "stack-1",
			Strategy: strategy,
		})
		require.NoError(t, err)
		require.Equal(t, stacks.SyncAlreadyUpToDate, res.Status)

		// Create a new commit on stack-1 so that we can test the actual update functionality
		gittest.WithCheckoutBranch(t, repo, "stack-1", func() {
			gittest.CommitFile(t, repo, "file1.txt", []byte("file1 updated"))
		})

		res, err = stacks.SyncBranch(repo, &stacks.SyncBranchOpts{
			Parent:   "stack-1",
			Strategy: strategy,
		})
		require.NoError(t, err)
		require.Equal(t, stacks.SyncUpdated, res.Status)

		data, err := ioutil.ReadFile(path.Join(repo.Dir(), "file1.txt"))
		require.NoError(t, err)
		require.Equal(t, "file1 updated", string(data), "file1 should have been updated after stack sync")
	}
}

func TestSyncBranch_WithConflicts(t *testing.T) {
	for _, strategy := range []stacks.SyncStrategy{
		stacks.StrategyMergeCommit,
		stacks.StrategyRebase,
	} {
		repo := gittest.NewTempRepo(t)

		_, err := repo.CheckoutBranch(&git.CheckoutBranch{Name: "stack-1", NewBranch: true})
		require.NoError(t, err)
		gittest.CommitFile(t, repo, "file.txt", []byte("commit 1a\n"))

		_, err = repo.CheckoutBranch(&git.CheckoutBranch{Name: "stack-2", NewBranch: true})
		require.NoError(t, err)
		gittest.CommitFile(t, repo, "file.txt", []byte("commit 1a\ncommit 2a\n"))

		gittest.WithCheckoutBranch(t, repo, "stack-1", func() {
			gittest.CommitFile(t, repo, "file.txt", []byte("commit 1a\ncommit 1b\n"))
		})

		res, err := stacks.SyncBranch(repo, &stacks.SyncBranchOpts{
			Parent:   "stack-1",
			Strategy: strategy,
		})
		require.NoError(t, err)
		require.Equal(t, stacks.SyncConflict, res.Status)
	}
}
