package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
)

func TestStackAdopt_Success_NoStackRoot(t *testing.T) {
	t.Skip("Known issue. Not fixed yet.")

	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	// Create stacked branches without av, and then adopt them.
	// main: root_commit
	// stack-1: root_commit -> 1a -> 1b
	// stack-2: root_commit -> 1a -> 1b -> 2a -> 2b
	// stack-3: root_commit -> 1a -> 1b -> 2a -> 2b -> 3a -> 3b
	// stack-4: root_commit -> 1a -> 1b -> 4a -> 4b
	repo.Git(t, "checkout", "-b", "stack-1")
	repo.CommitFile(t, "my-file", "1a\n", gittest.WithMessage("Commit 1a"))
	repo.CommitFile(t, "my-file", "1b\n", gittest.WithMessage("Commit 1b"))
	repo.Git(t, "checkout", "-b", "stack-2")
	repo.CommitFile(t, "my-file", "2a\n", gittest.WithMessage("Commit 2a"))
	repo.CommitFile(t, "my-file", "2b\n", gittest.WithMessage("Commit 2b"))
	repo.Git(t, "checkout", "-b", "stack-3")
	repo.CommitFile(t, "my-file", "3a\n", gittest.WithMessage("Commit 3a"))
	repo.CommitFile(t, "my-file", "3b\n", gittest.WithMessage("Commit 3b"))
	repo.Git(t, "switch", "stack-1")
	repo.Git(t, "checkout", "-b", "stack-4")
	repo.CommitFile(t, "my-file", "4a\n", gittest.WithMessage("Commit 4a"))
	repo.CommitFile(t, "my-file", "4b\n", gittest.WithMessage("Commit 4b"))

	repo.Git(t, "switch", "main")
	RequireAv(t, "stack", "adopt", "--dry-run")
}

func TestStackAdopt_Success_WithStackRoot(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	// main: root_commit
	// stack-1: root_commit -> 1a -> 1b
	RequireAv(t, "stack", "branch", "stack-1")
	repo.CommitFile(t, "my-file", "1a\n", gittest.WithMessage("Commit 1a"))
	repo.CommitFile(t, "my-file", "1b\n", gittest.WithMessage("Commit 1b"))

	// Create stacked branches without av, and then adopt them.
	// stack-2: root_commit -> 1a -> 1b -> 2a -> 2b
	// stack-3: root_commit -> 1a -> 1b -> 2a -> 2b -> 3a -> 3b
	// stack-4: root_commit -> 1a -> 1b -> 4a -> 4b
	repo.Git(t, "checkout", "-b", "stack-2")
	repo.CommitFile(t, "my-file", "2a\n", gittest.WithMessage("Commit 2a"))
	repo.CommitFile(t, "my-file", "2b\n", gittest.WithMessage("Commit 2b"))
	repo.Git(t, "checkout", "-b", "stack-3")
	repo.CommitFile(t, "my-file", "3a\n", gittest.WithMessage("Commit 3a"))
	repo.CommitFile(t, "my-file", "3b\n", gittest.WithMessage("Commit 3b"))
	repo.Git(t, "switch", "stack-1")
	repo.Git(t, "checkout", "-b", "stack-4")
	repo.CommitFile(t, "my-file", "4a\n", gittest.WithMessage("Commit 4a"))
	repo.CommitFile(t, "my-file", "4b\n", gittest.WithMessage("Commit 4b"))

	repo.Git(t, "switch", "stack-1")
	RequireAv(t, "stack", "adopt", "--dry-run")
}
