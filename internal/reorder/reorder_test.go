package reorder_test

import (
	"fmt"
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/reorder"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReorder(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	db := repo.OpenDB(t)

	initial := repo.GetCommitAtRef(t, plumbing.HEAD)

	repo.CreateRef(t, plumbing.NewBranchReferenceName("one"))
	repo.CheckoutBranch(t, plumbing.NewBranchReferenceName("one"))
	c1a := repo.CommitFile(t, "file", "hello\n")
	c1b := repo.CommitFile(t, "file", "hello\nworld\n")
	c2a := repo.CommitFile(t, "fichier", "bonjour\n")
	c2b := repo.CommitFile(t, "fichier", "bonjour\nle monde\n")

	continuation, err := reorder.Reorder(reorder.Context{
		Repo: repo.AsAvGitRepo(),
		DB:   db,
		State: &reorder.State{
			Branch: "",
			Head:   "",
			Commands: []reorder.Cmd{
				reorder.StackBranchCmd{Name: "one", Trunk: fmt.Sprintf("main@%s", initial)},
				reorder.PickCmd{Commit: c1a.String()},
				reorder.PickCmd{Commit: c1b.String()},
				reorder.StackBranchCmd{Name: "two", Parent: "one"},
				reorder.PickCmd{Commit: c2a.String()},
				reorder.PickCmd{Commit: c2b.String()},
			},
		},
	})
	require.NoError(t, err, "expected reorder to complete cleanly")
	require.Nil(t, continuation, "expected reorder to complete cleanly")

	mainHead := repo.GetCommitAtRef(t, plumbing.NewBranchReferenceName("main"))
	assert.Equal(t, initial, mainHead, "expected main to be at initial commit")

	oneHead := repo.GetCommitAtRef(t, plumbing.NewBranchReferenceName("one"))
	assert.Equal(t, c1b, oneHead, "expected one to be at c1b")

	twoHead := repo.GetCommitAtRef(t, plumbing.NewBranchReferenceName("two"))
	assert.Equal(t, c2b, twoHead, "expected two to be at c2b")
}

func TestReorderConflict(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	db := repo.OpenDB(t)

	initial := repo.GetCommitAtRef(t, plumbing.HEAD)

	repo.CreateRef(t, plumbing.NewBranchReferenceName("one"))
	repo.CheckoutBranch(t, plumbing.NewBranchReferenceName("one"))
	c1a := repo.CommitFile(t, "file", "hello\n")
	c1b := repo.CommitFile(t, "file", "hello\nworld\n")

	repo.Git(t, "reset", "--hard", initial.String())
	c2a := repo.CommitFile(t, "file", "bonjour\n")
	c2b := repo.CommitFile(t, "file", "bonjour\nle monde\n")

	continuation, err := reorder.Reorder(reorder.Context{
		Repo: repo.AsAvGitRepo(),
		DB:   db,
		State: &reorder.State{
			Branch: "",
			Head:   "",
			Commands: []reorder.Cmd{
				reorder.StackBranchCmd{Name: "one", Trunk: fmt.Sprintf("main@%s", initial)},
				reorder.PickCmd{Commit: c1a.String()},
				reorder.PickCmd{Commit: c1b.String()},
				reorder.PickCmd{Commit: c2a.String()},
				reorder.PickCmd{Commit: c2b.String()},
			},
		},
	})
	require.NoError(t, err, "expected reorder to complete without error even with conflicts")
	require.NotNil(t, continuation, "expected continuation to be returned after conflicts")
	require.Equal(t, continuation.State.Commands[0], reorder.PickCmd{Commit: c2a.String()})
}
