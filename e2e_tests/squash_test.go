package e2e_tests

import (
	"strings"
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
)

func TestSquashBasic(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)
	repo.Git(t, "fetch")

	// Create a branch with multiple commits
	repo.CreateFile(t, "file1.txt", "content1")
	repo.AddFile(t, "file1.txt")
	RequireAv(t, "branch", "feature-branch")
	RequireAv(t, "commit", "-m", "first commit")

	// Add second commit
	repo.CreateFile(t, "file2.txt", "content2")
	repo.AddFile(t, "file2.txt")
	RequireAv(t, "commit", "-m", "second commit")

	// Add third commit
	repo.CreateFile(t, "file3.txt", "content3")
	repo.AddFile(t, "file3.txt")
	RequireAv(t, "commit", "-m", "third commit")

	// Verify we have multiple commits
	commits := repo.Git(t, "rev-list", "--count", "feature-branch", "^main")
	require.Equal(t, "3\n", commits)

	// Squash the commits
	output := RequireAv(t, "squash")
	require.Contains(t, output.Stderr, "Successfully squashed 3 commits")

	// Now we should have only 1 commit after squashing all into the first commit
	commits = repo.Git(t, "rev-list", "--count", "feature-branch", "^main")
	require.Equal(t, "1\n", commits)

	// Verify all files are still present
	require.FileExists(t, repo.RepoDir+"/file1.txt")
	require.FileExists(t, repo.RepoDir+"/file2.txt")
	require.FileExists(t, repo.RepoDir+"/file3.txt")
}

func TestSquashWithTwoCommits(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)
	repo.Git(t, "fetch")

	repo.CreateFile(t, "base.txt", "content1")
	repo.AddFile(t, "base.txt")
	repo.CommitFile(t, "base.txt", "base commit")

	RequireAv(t, "branch", "feature-branch")
	repo.CreateFile(t, "file1.txt", "content1")
	repo.AddFile(t, "file1.txt")
	RequireAv(t, "commit", "-m", "first commit")

	repo.CreateFile(t, "file2.txt", "content2")
	repo.AddFile(t, "file2.txt")
	RequireAv(t, "commit", "-m", "second commit")

	// Squash the commits
	output := RequireAv(t, "squash")
	require.Contains(t, output.Stderr, "Successfully squashed 2 commits")

	// Now we should have only 1 commit after squashing both commits into the first
	commits := repo.Git(t, "rev-list", "--count", "feature-branch", "^main")
	require.Equal(t, "1\n", commits)
}

func TestSquashFailsWithSingleCommit(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)
	repo.Git(t, "fetch")

	// Create a branch with only one commit
	repo.CreateFile(t, "file1.txt", "content1")
	repo.AddFile(t, "file1.txt")
	RequireAv(t, "branch", "feature-branch")
	RequireAv(t, "commit", "-m", "only commit")

	// Squash should fail
	output := Av(t, "squash")
	require.NotEqual(t, 0, output.ExitCode)
	require.Contains(t, output.Stderr, "no commits to squash")
}

func TestSquashFailsWithNoCommits(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)
	repo.Git(t, "fetch")

	// Create a branch with no commits (same as main)
	RequireAv(t, "branch", "feature-branch")

	// Squash should fail
	output := Av(t, "squash")
	require.NotEqual(t, 0, output.ExitCode)
	require.Contains(t, output.Stderr, "no commits to squash")
}

func TestSquashFailsWithDirtyWorkingDirectory(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)
	repo.Git(t, "fetch")

	// Create a branch with multiple commits
	repo.CreateFile(t, "file1.txt", "content1")
	repo.AddFile(t, "file1.txt")
	RequireAv(t, "branch", "feature-branch")
	RequireAv(t, "commit", "-m", "first commit")

	repo.CreateFile(t, "file2.txt", "content2")
	repo.AddFile(t, "file2.txt")
	RequireAv(t, "commit", "-m", "second commit")

	// Create uncommitted changes
	repo.CreateFile(t, "dirty.txt", "uncommitted changes")

	// Squash should fail due to dirty working directory
	output := Av(t, "squash")
	require.NotEqual(t, 0, output.ExitCode)
	require.Contains(t, output.Stderr, "the working directory is not clean")
}

func TestSquashFailsOnUnmanagedBranch(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)
	repo.Git(t, "fetch")

	// Create a branch using git directly (not managed by av)
	repo.Git(t, "checkout", "-b", "unmanaged-branch")
	repo.CreateFile(t, "file1.txt", "content1")
	repo.AddFile(t, "file1.txt")
	repo.Git(t, "commit", "-m", "first commit")

	repo.CreateFile(t, "file2.txt", "content2")
	repo.AddFile(t, "file2.txt")
	repo.Git(t, "commit", "-m", "second commit")

	// Squash should fail because branch is not in database
	output := Av(t, "squash")
	require.NotEqual(t, 0, output.ExitCode)
	require.Contains(t, output.Stderr, "current branch does not exist in the database")
}

func TestSquashPreservesCommitMessage(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)
	repo.Git(t, "fetch")

	// Create a branch with multiple commits
	repo.CreateFile(t, "file1.txt", "content1")
	repo.AddFile(t, "file1.txt")
	RequireAv(t, "branch", "feature-branch")
	RequireAv(t, "commit", "-m", "first commit message")

	repo.CreateFile(t, "file2.txt", "content2")
	repo.AddFile(t, "file2.txt")
	RequireAv(t, "commit", "-m", "second commit message")

	// Squash the commits
	RequireAv(t, "squash")

	// Verify the first commit message is preserved (since we squash into the first commit)
	message := repo.Git(t, "log", "--format=%s", "-1")
	require.Equal(t, "first commit message\n", message)
}

func TestSquashWithStackedBranches(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)
	repo.Git(t, "fetch")

	// Create branch A with multiple commits
	RequireAv(t, "branch", "branch-a")
	repo.CreateFile(t, "a1.txt", "content a1")
	repo.AddFile(t, "a1.txt")
	RequireAv(t, "commit", "-m", "branch A commit 1")

	repo.CreateFile(t, "a2.txt", "content a2")
	repo.AddFile(t, "a2.txt")
	RequireAv(t, "commit", "-m", "branch A commit 2")

	repo.CreateFile(t, "a3.txt", "content a3")
	repo.AddFile(t, "a3.txt")
	RequireAv(t, "commit", "-m", "branch A commit 3")

	// Create branch B on top of branch A
	RequireAv(t, "branch", "branch-b")
	repo.CreateFile(t, "b1.txt", "content b1")
	repo.AddFile(t, "b1.txt")
	RequireAv(t, "commit", "-m", "branch B commit 1")

	repo.CreateFile(t, "b2.txt", "content b2")
	repo.AddFile(t, "b2.txt")
	RequireAv(t, "commit", "-m", "branch B commit 2")

	// Verify initial state
	// Branch A should have 3 commits
	RequireAv(t, "switch", "branch-a")
	commitsA := repo.Git(t, "rev-list", "--count", "branch-a", "^main")
	require.Equal(t, "3\n", commitsA)

	// Branch B should have 5 commits total (3 from A + 2 from B)
	RequireAv(t, "switch", "branch-b")
	commitsB := repo.Git(t, "rev-list", "--count", "branch-b", "^main")
	require.Equal(t, "5\n", commitsB)

	// Squash branch A (while on branch A)
	RequireAv(t, "switch", "branch-a")
	output := RequireAv(t, "squash")
	require.Contains(t, output.Stderr, "Successfully squashed 3 commits")

	// Branch A should now have 1 commit
	commitsA = repo.Git(t, "rev-list", "--count", "branch-a", "^main")
	require.Equal(t, "1\n", commitsA)

	// Branch B should still exist and be functional
	RequireAv(t, "switch", "branch-b")

	// All files should still be present in both branches
	require.FileExists(t, repo.RepoDir+"/a1.txt")
	require.FileExists(t, repo.RepoDir+"/a2.txt")
	require.FileExists(t, repo.RepoDir+"/a3.txt")
	require.FileExists(t, repo.RepoDir+"/b1.txt")
	require.FileExists(t, repo.RepoDir+"/b2.txt")

	// Branch B should still have its own commits on top of the squashed branch A
	// This should be 3 commits: 1 squashed commit from A + 2 commits from B
	commitsB = repo.Git(t, "rev-list", "--count", "branch-b", "^main")
	require.Equal(t, "3\n", commitsB)
}

func TestSquashStackedBranchBDoesNotAffectBranchA(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)
	repo.Git(t, "fetch")

	// Create branch A with multiple commits
	RequireAv(t, "branch", "branch-a")
	repo.CreateFile(t, "a1.txt", "content a1")
	repo.AddFile(t, "a1.txt")
	RequireAv(t, "commit", "-m", "branch A commit 1")

	repo.CreateFile(t, "a2.txt", "content a2")
	repo.AddFile(t, "a2.txt")
	RequireAv(t, "commit", "-m", "branch A commit 2")

	repo.CreateFile(t, "a3.txt", "content a3")
	repo.AddFile(t, "a3.txt")
	RequireAv(t, "commit", "-m", "branch A commit 3")

	// Create branch B on top of branch A with its own commits
	RequireAv(t, "branch", "branch-b")
	repo.CreateFile(t, "b1.txt", "content b1")
	repo.AddFile(t, "b1.txt")
	RequireAv(t, "commit", "-m", "branch B commit 1")

	repo.CreateFile(t, "b2.txt", "content b2")
	repo.AddFile(t, "b2.txt")
	RequireAv(t, "commit", "-m", "branch B commit 2")

	repo.CreateFile(t, "b3.txt", "content b3")
	repo.AddFile(t, "b3.txt")
	RequireAv(t, "commit", "-m", "branch B commit 3")

	// Verify initial state
	// Branch A should have 3 commits
	RequireAv(t, "switch", "branch-a")
	commitsA := repo.Git(t, "rev-list", "--count", "branch-a", "^main")
	require.Equal(t, "3\n", commitsA)

	// Branch B should have 6 commits total (3 from A + 3 from B)
	RequireAv(t, "switch", "branch-b")
	commitsB := repo.Git(t, "rev-list", "--count", "branch-b", "^main")
	require.Equal(t, "6\n", commitsB)

	// Branch B should have 3 commits that are unique to it (not in branch A)
	commitsBOnlyB := repo.Git(t, "rev-list", "--count", "branch-b", "^branch-a")
	require.Equal(t, "3\n", commitsBOnlyB)

	// Squash branch B (while on branch B) - should only squash B's commits
	output := RequireAv(t, "squash")
	require.Contains(t, output.Stderr, "Successfully squashed 3 commits")

	// Verify branch A is unchanged - should still have 3 commits
	RequireAv(t, "switch", "branch-a")
	commitsA = repo.Git(t, "rev-list", "--count", "branch-a", "^main")
	require.Equal(t, "3\n", commitsA)

	// Branch A should still have all its individual commits (not squashed)
	commitMessagesA := repo.Git(t, "log", "--format=%s", "branch-a", "^main")
	require.Contains(t, commitMessagesA, "branch A commit 1")
	require.Contains(t, commitMessagesA, "branch A commit 2")
	require.Contains(t, commitMessagesA, "branch A commit 3")

	// Branch B should now have 4 commits total (3 from A + 1 squashed from B)
	RequireAv(t, "switch", "branch-b")
	commitsB = repo.Git(t, "rev-list", "--count", "branch-b", "^main")
	require.Equal(t, "4\n", commitsB)

	// Branch B should have 1 commit unique to it (the squashed commit)
	commitsBOnlyB = repo.Git(t, "rev-list", "--count", "branch-b", "^branch-a")
	require.Equal(t, "1\n", commitsBOnlyB)

	// Verify the squashed commit message is the first B commit (branch B commit 1)
	squashedMessage := repo.Git(t, "log", "--format=%s", "-1", "branch-b", "^branch-a")
	require.Equal(t, "branch B commit 1\n", squashedMessage)

	// All files should still be present
	require.FileExists(t, repo.RepoDir+"/a1.txt")
	require.FileExists(t, repo.RepoDir+"/a2.txt")
	require.FileExists(t, repo.RepoDir+"/a3.txt")
	require.FileExists(t, repo.RepoDir+"/b1.txt")
	require.FileExists(t, repo.RepoDir+"/b2.txt")
	require.FileExists(t, repo.RepoDir+"/b3.txt")
}

func TestSquashWithModifiedParentBranch(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)
	repo.Git(t, "fetch")

	// Create parent branch
	RequireAv(t, "branch", "parent-branch")
	repo.CreateFile(t, "parent1.txt", "parent content 1")
	repo.AddFile(t, "parent1.txt")
	RequireAv(t, "commit", "-m", "parent commit 1")

	repo.CreateFile(t, "parent2.txt", "parent content 2")
	repo.AddFile(t, "parent2.txt")
	RequireAv(t, "commit", "-m", "parent commit 2")

	// Create child branch from parent
	RequireAv(t, "branch", "child-branch")
	repo.CreateFile(t, "child1.txt", "child content 1")
	repo.AddFile(t, "child1.txt")
	RequireAv(t, "commit", "-m", "child commit 1")

	repo.CreateFile(t, "child2.txt", "child content 2")
	repo.AddFile(t, "child2.txt")
	RequireAv(t, "commit", "-m", "child commit 2")

	// Simulate external changes to parent branch (force push scenario)
	repo.Git(t, "checkout", "parent-branch")
	repo.Git(t, "reset", "--hard", "HEAD~1") // Reset to parent commit 1
	repo.CreateFile(t, "parent_new.txt", "new parent content")
	repo.AddFile(t, "parent_new.txt")
	repo.Git(t, "commit", "-m", "parent new commit")

	// Switch back to child branch and attempt squash
	RequireAv(t, "switch", "child-branch")

	// Squash should now fail because branch is not in sync with parent
	output := Av(t, "squash")
	require.NotEqual(t, 0, output.ExitCode)
	require.Contains(t, output.Stderr, "branch is not in sync with parent branch")
	require.Contains(t, output.Stderr, "please run 'av sync' first")

	// Verify child files are still present (no partial squash happened)
	require.FileExists(t, repo.RepoDir+"/child1.txt")
	require.FileExists(t, repo.RepoDir+"/child2.txt")

	// Verify we still have the expected commits (no squashing occurred)
	commitCount := strings.TrimSpace(
		repo.Git(t, "rev-list", "--count", "child-branch", "^parent-branch"),
	)
	// After the parent branch was force-pushed (reset and new commits), the child branch
	// now has more commits relative to the new parent head, which is expected behavior
	require.NotEqual(
		t,
		"1",
		commitCount,
		"Should have more than 1 commit, confirming squash was prevented",
	)

	// The key achievement: squash was prevented, avoiding the bug where we would
	// squash into a parent commit due to the force-pushed parent branch
}

func TestSquashFailsWhenBranchOutOfSync(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)
	repo.Git(t, "fetch")

	// Create parent branch
	RequireAv(t, "branch", "parent-branch")
	repo.CreateFile(t, "parent1.txt", "parent content 1")
	repo.AddFile(t, "parent1.txt")
	RequireAv(t, "commit", "-m", "parent commit 1")

	// Create child branch from parent
	RequireAv(t, "branch", "child-branch")
	repo.CreateFile(t, "child1.txt", "child content 1")
	repo.AddFile(t, "child1.txt")
	RequireAv(t, "commit", "-m", "child commit 1")

	repo.CreateFile(t, "child2.txt", "child content 2")
	repo.AddFile(t, "child2.txt")
	RequireAv(t, "commit", "-m", "child commit 2")

	// Modify parent branch externally to create out-of-sync condition
	repo.Git(t, "checkout", "parent-branch")
	repo.CreateFile(t, "parent2.txt", "parent content 2")
	repo.AddFile(t, "parent2.txt")
	repo.Git(t, "commit", "-m", "parent commit 2")

	// Switch back to child branch
	RequireAv(t, "switch", "child-branch")

	// Squash should fail because branch is not in sync with parent
	output := Av(t, "squash")
	require.NotEqual(t, 0, output.ExitCode)
	require.Contains(t, output.Stderr, "branch is not in sync with parent branch parent-branch")
	require.Contains(t, output.Stderr, "please run 'av sync' first")

	// Verify no changes were made (both child commits should still exist)
	commitCount := strings.TrimSpace(
		repo.Git(t, "rev-list", "--count", "child-branch", "^parent-branch"),
	)
	require.Equal(t, "2", commitCount)
}
