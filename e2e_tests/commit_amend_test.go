package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
)

func TestCommitAmendInStack(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)
	RequireCmd(t, "git", "fetch")
	initialTimestamp := GetFetchHeadTimestamp(t, repo)

	// Create a branch and commit a file.
	filepath := repo.CreateFile(t, "one.txt", "one")
	repo.AddFile(t, filepath)
	RequireAv(t, "stack", "branch", "one")
	RequireAv(t, "commit", "create", "-m", "one")

	// Create another branch and commit a file.
	filepath = repo.CreateFile(t, "two.txt", "two")
	repo.AddFile(t, filepath)
	RequireAv(t, "stack", "branch", "two")
	RequireAv(t, "commit", "create", "-m", "two")

	// Go back to the first branch and amend the commit with another file.
	RequireCmd(t, "git", "checkout", "one")
	filepath = repo.CreateFile(t, "one-b.txt", "one-b")
	repo.AddFile(t, filepath)
	RequireAv(t, "commit", "amend", "--no-edit")

	// Verify that the branches are still there.
	db := repo.OpenDB(t)
	branchNames := maps.Keys(db.ReadTx().AllBranches())
	require.ElementsMatch(t, branchNames, []string{"one", "two"})

	// Commit shouldn't have triggered a fetch.
	updatedTimestamp := GetFetchHeadTimestamp(t, repo)
	require.Equal(t, initialTimestamp, updatedTimestamp)

	// It also shouldn't have triggered a push.
	// TODO: once we support mocking the GitHub API and there is an associated PR,
	// validate that a push didn't happen.
}
