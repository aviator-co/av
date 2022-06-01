package e2e_tests

import (
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"testing"
)

func TestStackSyncReparent(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.Dir())

	RequireAv(t, "stack", "branch", "foo")
	gittest.CommitFile(t, repo, "foo.txt", []byte("foo"))
	requireFileContent(t, "foo.txt", "foo")

	RequireAv(t, "stack", "branch", "bar")
	gittest.CommitFile(t, repo, "bar.txt", []byte("bar"))
	requireFileContent(t, "bar.txt", "bar")
	requireFileContent(t, "foo.txt", "foo")

	RequireAv(t, "stack", "branch", "spam")
	gittest.CommitFile(t, repo, "spam.txt", []byte("spam"))
	requireFileContent(t, "spam.txt", "spam")

	// Now, re-parent spam on top of bar (should be relatively a no-op)
	RequireAv(t, "stack", "reparent", "bar")
	requireFileContent(t, "spam.txt", "spam")
	requireFileContent(t, "bar.txt", "bar", "bar.txt should still be set after reparenting onto same branch")

	// Now, re-parent spam on top of foo
	RequireAv(t, "stack", "reparent", "foo")
	currentBranch, err := repo.CurrentBranchName()
	require.NoError(t, err)
	require.Equal(t, "spam", currentBranch, "branch should be set to original branch (spam) after reparenting onto foo")
	requireFileContent(t, "spam.txt", "spam")
	requireFileContent(t, "foo.txt", "foo", "foo.txt should be set after reparenting onto foo branch")
	require.NoFileExists(t, "bar.txt", "bar.txt should not exist after reparenting onto foo branch")
}

func requireFileContent(t *testing.T, file string, expected string, args ...any) {
	actual, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, expected, string(actual), args...)
}
