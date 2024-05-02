package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
)

func TestCommitCreateInStack(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.Dir())
	RequireCmd(t, "git", "fetch")
	initialTimestamp := GetFetchHeadTimestamp(t, repo)

	// Create a branch and commit a file.
	filepath := gittest.CreateFile(t, repo, "one.txt", []byte("one"))
	gittest.AddFile(t, repo, filepath)
	RequireAv(t, "stack", "branch", "one")
	RequireAv(t, "commit", "create", "-m", "one")

	// Create another branch and commit a file.
	filepath = gittest.CreateFile(t, repo, "two.txt", []byte("two"))
	gittest.AddFile(t, repo, filepath)
	RequireAv(t, "stack", "branch", "two")
	RequireAv(t, "commit", "create", "-m", "two")

	// Go back to the first branch and commit another file.
	RequireCmd(t, "git", "checkout", "one")
	filepath = gittest.CreateFile(t, repo, "one-b.txt", []byte("one-b"))
	gittest.AddFile(t, repo, filepath)
	RequireAv(t, "commit", "create", "-m", "one-b")

	// Verify that the branches are still there.
	db, err := jsonfiledb.OpenRepo(repo)
	require.NoError(t, err, "failed to open repo db")
	branchNames := maps.Keys(db.ReadTx().AllBranches())
	require.ElementsMatch(t, branchNames, []string{"one", "two"})

	// Commit shouldn't have triggered a fetch.
	updatedTimestamp := GetFetchHeadTimestamp(t, repo)
	require.Equal(t, initialTimestamp, updatedTimestamp)

	// It also shouldn't have triggered a push.
	// TODO: once we support mocking the GitHub API and there is an associated PR,
	// validate that a push didn't happen.
}
