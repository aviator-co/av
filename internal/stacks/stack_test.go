package stacks_test

import (
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/stacks"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStackTree(t *testing.T) {
	repo := gittest.NewTempRepo(t)

	err := stacks.CreateBranch(repo, &stacks.CreateBranchOpts{
		Name: "1a",
	})
	require.NoError(t, err)
	gittest.CommitFile(t, repo, "1a.txt", []byte("hello i am 1a"))

	err = stacks.CreateBranch(repo, &stacks.CreateBranchOpts{
		Name: "1b",
	})
	require.NoError(t, err)
	gittest.CommitFile(t, repo, "1b.txt", []byte("hello i am 1b"))

	_, err = repo.CheckoutBranch(&git.CheckoutBranch{Name: "1a"})
	require.NoError(t, err, "failed to checkout branch 1a")
	err = stacks.CreateBranch(repo, &stacks.CreateBranchOpts{
		Name: "1c",
	})
	require.NoError(t, err)
	gittest.CommitFile(t, repo, "1c.txt", []byte("hello i am 1c"))

	trees, err := stacks.GetTrees(repo)
	require.NoError(t, err)
	require.Equal(t, 1, len(trees))

	tree := trees["1a"]
	require.NotNil(t, tree)
	require.Equal(t, "1a", tree.Branch.Name)
	require.Equal(t, 2, len(tree.Next))

	firstChild := tree.Next[0]
	if firstChild.Branch.Name != "1b" && firstChild.Branch.Name != "1c" {
		t.Errorf("expected 1b or 1c to be child, got %s", firstChild.Branch.Name)
	}

	secondChild := tree.Next[1]
	if secondChild.Branch.Name != "1b" && secondChild.Branch.Name != "1c" {
		t.Errorf("expected 1b or 1c to be child, got %s", secondChild.Branch.Name)
	}

	if firstChild.Branch.Name == secondChild.Branch.Name {
		t.Errorf("expected 1b and 1c to be different children")
	}
}
