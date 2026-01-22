package e2e_tests

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
)

// setupRepoWithPrefix creates a test repository with the specified branch name prefix.
func setupRepoWithPrefix(t *testing.T, prefix string) *gittest.GitTestRepo {
	t.Helper()
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	configDir := filepath.Join(repo.RepoDir, ".git", "av")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	configFile := filepath.Join(configDir, "config.yml")
	configContent := fmt.Sprintf("pullRequest:\n  branchNamePrefix: %q\n", prefix)
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0o644))

	return repo
}

func TestBranchNamePrefixWithCommit(t *testing.T) {
	repo := setupRepoWithPrefix(t, "user/myname/")

	// Create a branch using av commit -b
	repo.CreateFile(t, "test.txt", "test content")
	repo.AddFile(t, "test.txt")
	RequireAv(t, "commit", "-b", "-m", "test branch creation")

	// Verify the branch name has the prefix
	currentBranch := repo.Git(t, "branch", "--show-current")
	require.Equal(t, "user/myname/test-branch-creation\n", currentBranch)
}

func TestBranchNamePrefixWithSplit(t *testing.T) {
	repo := setupRepoWithPrefix(t, "feature/")

	// Create a branch and make two commits
	RequireAv(t, "branch", "base-branch")
	repo.CommitFile(t, "file1.txt", "content1")
	repo.CommitFile(t, "file2.txt", "content2")

	// Split the last commit
	RequireAv(t, "branch", "--split")

	// Verify the new branch name has the exact expected prefix and sanitized name
	currentBranch := repo.Git(t, "branch", "--show-current")
	require.Equal(t, "feature/write-file2-txt\n", currentBranch)
}

func TestBranchNamePrefixWithExplicitName(t *testing.T) {
	repo := setupRepoWithPrefix(t, "will/")

	// Create a branch with explicit name using av branch
	RequireAv(t, "branch", "test-branch")

	// Verify the branch name has the prefix applied
	currentBranch := repo.Git(t, "branch", "--show-current")
	require.Equal(t, "will/test-branch\n", currentBranch)
}

func TestBranchNamePrefixWithSplitExplicitName(t *testing.T) {
	repo := setupRepoWithPrefix(t, "user/dev/")

	// Create a branch and make two commits
	RequireAv(t, "branch", "base-branch")
	repo.CommitFile(t, "file1.txt", "content1")
	repo.CommitFile(t, "file2.txt", "content2")

	// Split the last commit with explicit name
	// Now the prefix SHOULD be applied to explicit names too for consistency
	RequireAv(t, "branch", "--split", "my-explicit-branch")

	// Verify the new branch name has the prefix applied
	currentBranch := repo.Git(t, "branch", "--show-current")
	require.Equal(t, "user/dev/my-explicit-branch\n", currentBranch)
}

func TestBranchNamePrefixEmpty(t *testing.T) {
	repo := setupRepoWithPrefix(t, "")

	repo.CreateFile(t, "test.txt", "test content")
	repo.AddFile(t, "test.txt")
	RequireAv(t, "commit", "-b", "-m", "test without prefix")

	// Verify the branch name has no prefix
	currentBranch := repo.Git(t, "branch", "--show-current")
	require.Equal(t, "test-without-prefix\n", currentBranch)
}
