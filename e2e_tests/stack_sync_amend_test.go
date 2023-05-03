package e2e_tests

import (
	"os"
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
)

func TestSyncAfterAmendingCommit(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.Dir())

	// Create a three stack...
	RequireCmd(t, "git", "checkout", "-b", "stack-1")
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n"), gittest.WithMessage("Commit 1a"))
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n1b\n"), gittest.WithMessage("Commit 1b"))
	RequireAv(t, "stack", "branch", "stack-2")
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n1b\n2a\n"), gittest.WithMessage("Commit 2a"))
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n1b\n2a\n2b\n"), gittest.WithMessage("Commit 2b"))
	RequireAv(t, "stack", "branch", "stack-3")
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n1b\n2a\n2b\n3a\n"), gittest.WithMessage("Commit 3a"))
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n1b\n2a\n2b\n3a\n3b\n"), gittest.WithMessage("Commit 3b"))

	// Now we amend commit 1b and make sure the sync after succeeds
	gittest.CheckoutBranch(t, repo, "stack-1")
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n1c\n1b\n"), gittest.WithAmend())
	sync := Av(t, "stack", "sync", "--no-fetch", "--no-push")
	require.Equal(t, 0, sync.ExitCode, "expected sync to succeed")
	gittest.CheckoutBranch(t, repo, "stack-3")
	contents, err := os.ReadFile("my-file")
	require.NoError(t, err)
	require.Equal(t, "1a\n1c\n1b\n2a\n2b\n3a\n3b\n", string(contents))

	// Now we amend commit 2a and make sure the sync succeeds
	gittest.CheckoutBranch(t, repo, "stack-2")
}
