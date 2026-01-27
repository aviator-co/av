package git_test

import (
	"strings"
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
)

func TestBranchSetUpstream(t *testing.T) {
	repo := gittest.NewTempRepo(t)

	// Create a test branch
	repo.Git(t, "checkout", "-b", "test-branch")
	repo.CommitFile(t, "test-file", "content\n", gittest.WithMessage("Test commit"))

	// Push to remote (without -u, so no upstream is set)
	repo.Git(t, "push", "origin", "test-branch")

	// Use BranchSetUpstream to set the upstream
	avRepo := repo.AsAvGitRepo()
	err := avRepo.BranchSetUpstream(t.Context(), "test-branch", "origin")
	require.NoError(t, err)

	// Verify upstream is now set by checking git config
	// git branch --set-upstream-to sets branch.<name>.remote and branch.<name>.merge
	remote := strings.TrimSpace(repo.Git(t, "config", "--get", "branch.test-branch.remote"))
	require.Equal(t, "origin", remote, "upstream remote should be set")

	merge := strings.TrimSpace(repo.Git(t, "config", "--get", "branch.test-branch.merge"))
	require.Equal(t, "refs/heads/test-branch", merge, "upstream merge ref should be set")

	// Also verify using git branch -vv which shows tracking info
	branchInfo := repo.Git(t, "branch", "-vv")
	require.Contains(t, branchInfo, "[origin/test-branch]", "branch should show tracking info")
}
