package e2e_tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
)

func TestBranchNamePrefixWithCommit(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	// Create a config file with BranchNamePrefix set
	configDir := filepath.Join(repo.RepoDir, ".git", "av")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	configFile := filepath.Join(configDir, "config.yml")
	configContent := `pullRequest:
  branchNamePrefix: "user/myname/"
`
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0o644))

	// Create a branch using av commit -b
	repo.CreateFile(t, "test.txt", "test content")
	repo.AddFile(t, "test.txt")
	RequireAv(t, "commit", "-b", "-m", "test branch creation")

	// Verify the branch name has the prefix
	currentBranch := repo.Git(t, "branch", "--show-current")
	require.Equal(t, "user/myname/test-branch-creation\n", currentBranch)
}

func TestBranchNamePrefixWithSplit(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	// Create a config file with BranchNamePrefix set
	configDir := filepath.Join(repo.RepoDir, ".git", "av")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	configFile := filepath.Join(configDir, "config.yml")
	configContent := `pullRequest:
  branchNamePrefix: "feature/"
`
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0o644))

	// Create a branch and make two commits
	RequireAv(t, "branch", "base-branch")
	repo.CommitFile(t, "file1.txt", "content1")
	repo.CommitFile(t, "file2.txt", "content2")

	// Split the last commit
	RequireAv(t, "branch", "--split")

	// Verify the new branch name has the prefix
	currentBranch := repo.Git(t, "branch", "--show-current")
	require.Contains(t, currentBranch, "feature/")
}

func TestBranchNamePrefixWithExplicitName(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	// Create a config file with BranchNamePrefix set
	configDir := filepath.Join(repo.RepoDir, ".git", "av")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	configFile := filepath.Join(configDir, "config.yml")
	configContent := `pullRequest:
  branchNamePrefix: "will/"
`
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0o644))

	// Create a branch with explicit name using av branch
	RequireAv(t, "branch", "test-branch")

	// Verify the branch name has the prefix applied
	currentBranch := repo.Git(t, "branch", "--show-current")
	require.Equal(t, "will/test-branch\n", currentBranch)
}

func TestBranchNamePrefixWithSplitExplicitName(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	// Create a config file with BranchNamePrefix set
	configDir := filepath.Join(repo.RepoDir, ".git", "av")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	configFile := filepath.Join(configDir, "config.yml")
	configContent := `pullRequest:
  branchNamePrefix: "user/dev/"
`
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0o644))

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
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	// Explicitly set empty BranchNamePrefix to override global config
	configDir := filepath.Join(repo.RepoDir, ".git", "av")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	configFile := filepath.Join(configDir, "config.yml")
	configContent := `pullRequest:
  branchNamePrefix: ""
`
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0o644))

	repo.CreateFile(t, "test.txt", "test content")
	repo.AddFile(t, "test.txt")
	RequireAv(t, "commit", "-b", "-m", "test without prefix")

	// Verify the branch name has no prefix
	currentBranch := repo.Git(t, "branch", "--show-current")
	require.Equal(t, "test-without-prefix\n", currentBranch)
}
