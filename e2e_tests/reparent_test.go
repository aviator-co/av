package e2e_tests

import (
	"os"
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"
)

func TestReparent(t *testing.T) {
	server := RunMockGitHubServer(t)
	defer server.Close()
	repo := gittest.NewTempRepoWithGitHubServer(t, server.URL)
	Chdir(t, repo.RepoDir)

	RequireAv(t, "branch", "foo")
	repo.CommitFile(t, "foo.txt", "foo")
	requireFileContent(t, "foo.txt", "foo")

	RequireAv(t, "branch", "bar")
	repo.CommitFile(t, "bar.txt", "bar")
	requireFileContent(t, "bar.txt", "bar")
	requireFileContent(t, "foo.txt", "foo")

	RequireAv(t, "branch", "spam")
	repo.CommitFile(t, "spam.txt", "spam")
	requireFileContent(t, "spam.txt", "spam")

	// Now, re-parent spam on top of bar (should be relatively a no-op)
	RequireAv(t, "reparent", "--parent", "bar")
	requireFileContent(t, "spam.txt", "spam")
	requireFileContent(
		t,
		"bar.txt",
		"bar",
		"bar.txt should still be set after reparenting onto same branch",
	)

	// Now, re-parent spam on top of foo
	RequireAv(t, "reparent", "--parent", "foo")
	currentBranch := repo.CurrentBranch(t)
	require.Equal(
		t,
		plumbing.ReferenceName("refs/heads/spam"),
		currentBranch,
		"branch should be set to original branch (spam) after reparenting onto foo",
	)
	requireFileContent(t, "spam.txt", "spam")
	requireFileContent(
		t,
		"foo.txt",
		"foo",
		"foo.txt should be set after reparenting onto foo branch",
	)
	require.NoFileExists(t, "bar.txt", "bar.txt should not exist after reparenting onto foo branch")
}

func TestReparentTrunk(t *testing.T) {
	server := RunMockGitHubServer(t)
	defer server.Close()
	repo := gittest.NewTempRepoWithGitHubServer(t, server.URL)
	Chdir(t, repo.RepoDir)

	RequireAv(t, "branch", "foo")
	repo.CommitFile(t, "foo.txt", "foo")

	RequireAv(t, "branch", "bar")
	repo.CommitFile(t, "bar.txt", "bar")

	// Delete the local main. av should use the remote tracking branch.
	repo.Git(t, "branch", "-D", "main")

	RequireAv(t, "reparent", "--parent", "main")
}

func requireFileContent(t *testing.T, file string, expected string, args ...any) {
	t.Helper()
	actual, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, expected, string(actual), args...)
}
