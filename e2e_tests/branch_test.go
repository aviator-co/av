package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta"
	"github.com/stretchr/testify/require"
)

func TestBranchMove(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	// Create one -> two -> three stack
	RequireAv(t, "branch", "one")
	repo.CommitFile(t, "one.txt", "one")
	RequireAv(t, "branch", "two")
	repo.CommitFile(t, "two.txt", "two")
	RequireAv(t, "branch", "three")
	repo.CommitFile(t, "three.txt", "three")

	// one -> un
	repo.Git(t, "checkout", "one")
	RequireAv(t, "branch", "-m", "un")
	RequireCurrentBranchName(t, repo, "refs/heads/un")

	// two -> deux
	// use "av next" here to make sure the parent child relationship is
	// correct
	RequireAv(t, "next")
	RequireCurrentBranchName(t, repo, "refs/heads/two")
	RequireAv(t, "branch", "-m", "deux")
	RequireCurrentBranchName(t, repo, "refs/heads/deux")

	// three -> trois
	RequireAv(t, "next")
	RequireCurrentBranchName(t, repo, "refs/heads/three")
	RequireAv(t, "branch", "-m", "trois")
	RequireCurrentBranchName(t, repo, "refs/heads/trois")

	// Make sure we've handled all the parent/child renames correctly
	db := repo.OpenDB(t)
	branches := db.ReadTx().AllBranches()
	require.Equal(t, true, branches["un"].Parent.Trunk, "expected parent(un) to be a trunk")
	require.Equal(
		t,
		[]string{"deux"},
		meta.ChildrenNames(db.ReadTx(), "un"),
		"expected un to have children [deux]",
	)
	require.Equal(t, "un", branches["deux"].Parent.Name, "expected parent(deux) to be un")
	require.Equal(
		t,
		[]string{"trois"},
		meta.ChildrenNames(db.ReadTx(), "deux"),
		"expected deux to have children [trois]",
	)
	require.Equal(t, "deux", branches["trois"].Parent.Name, "expected parent(trois) to be deux")
	require.Len(t, meta.Children(db.ReadTx(), "trois"), 0, "expected trois to have no children")
}

func TestBranchRetroactiveMove(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	// Create one -> two -> three stack
	RequireAv(t, "branch", "one")
	repo.CommitFile(t, "one.txt", "one")
	RequireAv(t, "branch", "two")
	repo.CommitFile(t, "two.txt", "two")
	RequireAv(t, "branch", "three")
	repo.CommitFile(t, "three.txt", "three")

	// one -> un without av
	repo.Git(t, "checkout", "one")
	repo.Git(t, "branch", "-m", "un")
	RequireCurrentBranchName(t, repo, "refs/heads/un")

	// Retroactive rename with av
	RequireAv(t, "branch", "--rename", "one:un")

	// Make sure we've handled all the parent/child renames correctly
	db := repo.OpenDB(t)
	branches := db.ReadTx().AllBranches()
	require.Equal(t, true, branches["un"].Parent.Trunk, "expected parent(un) to be a trunk")
	require.Equal(
		t,
		[]string{"two"},
		meta.ChildrenNames(db.ReadTx(), "un"),
		"expected un to have children [two]",
	)
	require.NotContainsf(t, branches, "one", "expected one to be deleted from the branch metadata")
}
