package reorder_test

import (
	"fmt"
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/aviator-co/av/internal/reorder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReorder(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	db, err := jsonfiledb.OpenRepo(repo)
	require.NoError(t, err)

	initial, err := repo.RevParse(&git.RevParse{Rev: "HEAD"})
	require.NoError(t, err)

	_, err = repo.CheckoutBranch(&git.CheckoutBranch{Name: "one", NewBranch: true})
	require.NoError(t, err)
	c1a := gittest.CommitFile(t, repo, "file", []byte("hello\n"))
	c1b := gittest.CommitFile(t, repo, "file", []byte("hello\nworld\n"))
	c2a := gittest.CommitFile(t, repo, "fichier", []byte("bonjour\n"))
	c2b := gittest.CommitFile(t, repo, "fichier", []byte("bonjour\nle monde\n"))

	continuation, err := reorder.Reorder(reorder.Context{
		Repo: repo,
		DB:   db,
		State: &reorder.State{
			Branch: "",
			Head:   "",
			Commands: []reorder.Cmd{
				reorder.StackBranchCmd{Name: "one", Trunk: fmt.Sprintf("main@%s", initial)},
				reorder.PickCmd{Commit: c1a},
				reorder.PickCmd{Commit: c1b},
				reorder.StackBranchCmd{Name: "two", Parent: "one"},
				reorder.PickCmd{Commit: c2a},
				reorder.PickCmd{Commit: c2b},
			},
		},
	})
	require.NoError(t, err, "expected reorder to complete cleanly")
	require.Nil(t, continuation, "expected reorder to complete cleanly")

	mainHead, err := repo.RevParse(&git.RevParse{Rev: "main"})
	require.NoError(t, err)
	assert.Equal(t, initial, mainHead, "expected main to be at initial commit")

	oneHead, err := repo.RevParse(&git.RevParse{Rev: "one"})
	require.NoError(t, err)
	assert.Equal(t, c1b, oneHead, "expected one to be at c1b")

	twoHead, err := repo.RevParse(&git.RevParse{Rev: "two"})
	require.NoError(t, err)
	assert.Equal(t, c2b, twoHead, "expected two to be at c2b")
}

func TestReorderConflict(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	db, err := jsonfiledb.OpenRepo(repo)
	require.NoError(t, err)

	initial, err := repo.RevParse(&git.RevParse{Rev: "HEAD"})
	require.NoError(t, err)

	_, err = repo.CheckoutBranch(&git.CheckoutBranch{Name: "one", NewBranch: true})
	require.NoError(t, err)
	c1a := gittest.CommitFile(t, repo, "file", []byte("hello\n"))
	c1b := gittest.CommitFile(t, repo, "file", []byte("hello\nworld\n"))

	_, err = repo.Git("reset", "--hard", initial)
	require.NoError(t, err)
	c2a := gittest.CommitFile(t, repo, "file", []byte("bonjour\n"))
	c2b := gittest.CommitFile(t, repo, "file", []byte("bonjour\nle monde\n"))

	continuation, err := reorder.Reorder(reorder.Context{
		Repo: repo,
		DB:   db,
		State: &reorder.State{
			Branch: "",
			Head:   "",
			Commands: []reorder.Cmd{
				reorder.StackBranchCmd{Name: "one", Trunk: fmt.Sprintf("main@%s", initial)},
				reorder.PickCmd{Commit: c1a},
				reorder.PickCmd{Commit: c1b},
				reorder.PickCmd{Commit: c2a},
				reorder.PickCmd{Commit: c2b},
			},
		},
	})
	require.NoError(t, err, "expected reorder to complete without error even with conflicts")
	require.NotNil(t, continuation, "expected continuation to be returned after conflicts")
	require.Equal(t, continuation.State.Commands[0], reorder.PickCmd{Commit: c2a})
}
