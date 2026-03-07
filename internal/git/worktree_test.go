package git_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
)

func TestWorktreeForBranch(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	avRepo := repo.AsAvGitRepo()

	// Create a feature branch with a commit.
	repo.Git(t, "checkout", "-b", "feature-1")
	repo.CommitFile(t, "feature1.txt", "feature 1")
	repo.Git(t, "checkout", "main")

	// feature-1 is not checked out in any worktree, so should return empty.
	wt, err := avRepo.WorktreeForBranch(t.Context(), "feature-1")
	require.NoError(t, err)
	require.Empty(t, wt)

	// Add a worktree for feature-1.
	wtPath := t.TempDir()
	repo.Git(t, "worktree", "add", wtPath, "feature-1")

	// Now feature-1 should be detected in the worktree.
	wt, err = avRepo.WorktreeForBranch(t.Context(), "feature-1")
	require.NoError(t, err)
	// Resolve symlinks for comparison (macOS /var -> /private/var).
	resolvedWt, _ := filepath.EvalSymlinks(wt)
	resolvedExpected, _ := filepath.EvalSymlinks(wtPath)
	require.Equal(t, resolvedExpected, resolvedWt)

	// main is checked out in the main repo, not a different worktree.
	wt, err = avRepo.WorktreeForBranch(t.Context(), "main")
	require.NoError(t, err)
	require.Empty(t, wt)
}

func TestRebaseInWorktree(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	avRepo := repo.AsAvGitRepo()

	// Create a stack: main -> feature-1 -> feature-2
	repo.Git(t, "checkout", "-b", "feature-1")
	repo.CommitFile(t, "feature1.txt", "feature 1")
	repo.Git(t, "checkout", "-b", "feature-2")
	repo.CommitFile(t, "feature2.txt", "feature 2")
	repo.Git(t, "checkout", "main")

	// Add a new commit on main to create something to restack onto.
	repo.CommitFile(t, "main-update.txt", "main update")

	// Put feature-1 in a separate worktree.
	wtPath := t.TempDir()
	repo.Git(t, "worktree", "add", wtPath, "feature-1")

	// Rebase feature-1 onto main (simulating restack).
	// This would fail without worktree-aware rebase because feature-1
	// is checked out in another worktree.
	mainHash := strings.TrimSpace(repo.Git(t, "rev-parse", "main"))

	// Get the merge-base for feature-1 and main (the original branch point).
	mergeBase := strings.TrimSpace(repo.Git(t, "merge-base", "main", "feature-1"))

	result, err := avRepo.RebaseParse(t.Context(), git.RebaseOpts{
		Branch:   "feature-1",
		Upstream: mergeBase,
		Onto:     mainHash,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	// The rebase should succeed (updated or already up to date).
	require.NotEqual(t, git.RebaseConflict, result.Status)
}
