package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
)

func TestCommitCreateInStack(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)
	repo.Git(t, "fetch")
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

	// Go back to the first branch and commit another file.
	repo.Git(t, "checkout", "one")
	filepath = repo.CreateFile(t, "one-b.txt", "one-b")
	repo.AddFile(t, filepath)
	RequireAv(t, "commit", "create", "-m", "one-b")

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

func TestCommitCreateOnMergedBranch(t *testing.T) {
	server := RunMockGitHubServer(t)
	defer server.Close()
	repo := gittest.NewTempRepoWithGitHubServer(t, server.URL)
	Chdir(t, repo.RepoDir)
	repo.Git(t, "fetch")

	// Create a branch
	RequireAv(t, "stack", "branch", "one")

	// Simulate a merge state on the mock server
	server.pulls = append(server.pulls, mockPR{
		ID:          "nodeid-42",
		Number:      42,
		State:       "MERGED",
		HeadRefName: "one",
	})

	// Update the branch meta with the PR data
	db := repo.OpenDB(t)
	tx := db.WriteTx()
	oneMeta, _ := tx.Branch("one")
	oneMeta.PullRequest = &meta.PullRequest{ID: "nodeid-42", Number: 42}
	tx.SetBranch(oneMeta)
	require.NoError(t, tx.Commit())

	// Attempt to commit to the "merged" branch
	filepath := repo.CreateFile(t, "one.txt", "one")
	repo.AddFile(t, filepath)
	output := Av(t, "commit", "create", "-m", "two")
	require.Equal(t, 1, output.ExitCode, "expected exit code 1")
}
