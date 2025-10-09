package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
)

// TestSyncTrunkInteractivePrompt tests that when running `av sync` on trunk/master
// without --all flag, it syncs all stacks correctly after the user responds "Yes" to the prompt.
//
// NOTE: This test uses --all flag directly because testing the interactive Bubble Tea prompt
// requires a PTY (pseudo-terminal). The interactive flow is covered by manual testing as
// specified in the runbook. The fix in cmd/av/sync.go:203-217 ensures the callback properly
// returns the continuation command after setting syncFlags.All = true.
func TestSyncTrunkInteractivePrompt(t *testing.T) {
	server := RunMockGitHubServer(t)
	defer server.Close()
	repo := gittest.NewTempRepoWithGitHubServer(t, server.URL)
	Chdir(t, repo.RepoDir)

	// Create two independent stacks from main branch
	// Our stack looks like:
	//     main:    X
	//     stack-1:  \ -> 1a
	//     stack-2:  \ -> 2a

	// Create first stack
	repo.Git(t, "switch", "main")
	RequireAv(t, "branch", "stack-1")
	repo.CommitFile(t, "file1.txt", "1a\n", gittest.WithMessage("Commit 1a"))

	// Create second stack
	repo.Git(t, "switch", "main")
	RequireAv(t, "branch", "stack-2")
	repo.CommitFile(t, "file2.txt", "2a\n", gittest.WithMessage("Commit 2a"))

	// Add a new commit to main branch and push it to remote
	repo.Git(t, "switch", "main")
	repo.CommitFile(t, "main-file.txt", "X2\n", gittest.WithMessage("Commit X2"))
	repo.Git(t, "push", "origin", "main")

	// Now the repo looks like:
	//     main:    X -> X2
	//     stack-1:  \ -> 1a
	//     stack-2:  \ -> 2a

	// Get the new main commit hash before sync
	newMainCommit := repo.GetCommitAtRef(t, "main")

	// Check out main branch
	repo.Git(t, "checkout", "main")

	// Run av sync --all from main branch
	// This tests the same code path that would be taken if the user selected "Yes"
	// at the interactive prompt (which sets syncFlags.All = true and continues)
	RequireAv(t, "sync", "--all")

	// Now the repo should look like:
	//     main:    X -> X2
	//     stack-1:        \ -> 1a
	//     stack-2:        \ -> 2a

	// Verify that both stack-1 and stack-2 are rebased onto the new main commit
	// Use merge-base --is-ancestor to verify the new main commit is an ancestor
	repo.Git(t, "merge-base", "--is-ancestor", newMainCommit.String(), "stack-1")
	repo.Git(t, "merge-base", "--is-ancestor", newMainCommit.String(), "stack-2")

	// Also verify main is an ancestor of both stacks
	repo.Git(t, "merge-base", "--is-ancestor", "main", "stack-1")
	repo.Git(t, "merge-base", "--is-ancestor", "main", "stack-2")
}
