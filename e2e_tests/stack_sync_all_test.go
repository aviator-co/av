package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
)

func TestStackSyncAll(t *testing.T) {
	server := RunMockGitHubServer(t)
	defer server.Close()
	repo := gittest.NewTempRepoWithGitHubServer(t, server.URL)
	Chdir(t, repo.RepoDir)

	repo.Git(t, "switch", "main")
	RequireAv(t, "stack", "branch", "stack-1")
	repo.CommitFile(t, "my-file", "1a\n", gittest.WithMessage("Commit 1a"))

	repo.Git(t, "switch", "main")
	RequireAv(t, "stack", "branch", "stack-2")
	repo.CommitFile(t, "my-file", "2a\n", gittest.WithMessage("Commit 2a"))

	repo.Git(t, "switch", "main")
	repo.CommitFile(t, "other-file", "X2\n", gittest.WithMessage("Commit X2"))
	repo.Git(t, "push", "origin", "main")

	//     main:    X  -> X2
	//     stack-1:  \ -> 1a
	//     stack-2:  \ -> 2a

	RequireAv(t, "sync", "--all")

	//     main:    X  -> X2
	//     stack-1:        \ -> 1a
	//     stack-2:        \ -> 2a

	// HEAD of main should be an ancestor of HEAD of stack-1
	repo.Git(t, "merge-base", "--is-ancestor", "main", "stack-1")
	// HEAD of main should be an ancestor of HEAD of stack-2
	repo.Git(t, "merge-base", "--is-ancestor", "main", "stack-2")
}
