package e2e_tests

import (
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStackBranchMove(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.Dir())

	// Create one -> two -> three stack
	RequireAv(t, "stack", "branch", "one")
	gittest.CommitFile(t, repo, "one.txt", []byte("one"))
	RequireAv(t, "stack", "branch", "two")
	gittest.CommitFile(t, repo, "two.txt", []byte("two"))
	RequireAv(t, "stack", "branch", "three")
	gittest.CommitFile(t, repo, "three.txt", []byte("three"))

	// one -> un
	RequireCmd(t, "git", "checkout", "one")
	RequireAv(t, "stack", "branch", "-m", "un")
	RequireCurrentBranchName(t, repo, "un")

	// two -> deux
	// use "av stack next" here to make sure the parent child relationship is
	// correct
	RequireAv(t, "stack", "next")
	RequireCurrentBranchName(t, repo, "two")
	RequireAv(t, "stack", "branch", "-m", "deux")
	RequireCurrentBranchName(t, repo, "deux")

	// three -> trois
	RequireAv(t, "stack", "next")
	RequireCurrentBranchName(t, repo, "three")
	RequireAv(t, "stack", "branch", "-m", "trois")
	RequireCurrentBranchName(t, repo, "trois")

	// Make sure we've handled all the parent/child renames correctly
	branches, err := meta.ReadAllBranches(repo)
	require.NoError(t, err)
	require.Equal(t, true, branches["un"].Parent.Trunk, "expected parent(un) to be a trunk")
	require.Equal(t, []string{"deux"}, branches["un"].Children, "expected un to have children [deux]")
	require.Equal(t, "un", branches["deux"].Parent.Name, "expected parent(deux) to be un")
	require.Equal(t, []string{"trois"}, branches["deux"].Children, "expected deux to have children [trois]")
	require.Equal(t, "deux", branches["trois"].Parent.Name, "expected parent(trois) to be deux")
	require.Len(t, branches["trois"].Children, 0, "expected trois to have no children")
}
