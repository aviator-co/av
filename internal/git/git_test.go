package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
)

func TestOrigin(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	repo.Git(t, "remote", "set-url", "origin", "https://github.com/aviator-co/av.git")
	origin, err := repo.AsAvGitRepo().Origin(t.Context())
	require.NoError(t, err)
	require.Equal(t, "aviator-co/av", origin.RepoSlug)

	repo.Git(t, "remote", "set-url", "origin", "git@github.com:aviator-co/av.git")
	require.NoError(t, err)
	origin, err = repo.AsAvGitRepo().Origin(t.Context())
	require.NoError(t, err)
	require.Equal(t, "aviator-co/av", origin.RepoSlug)
}

func TestTrunkBranches(t *testing.T) {
	repo := gittest.NewTempRepo(t)

	branches := repo.AsAvGitRepo().TrunkBranches()
	require.Equal(t, branches, []string{"main"})

	// add some branches to AdditionalTrunkBranches
	config.Av.AdditionalTrunkBranches = []string{"develop", "staging"}
	branches = repo.AsAvGitRepo().TrunkBranches()
	require.Equal(t, branches, []string{"main", "develop", "staging"})
}

// TestWorktreeStateIsolation verifies rebase detection and state-file I/O
// target the per-worktree git dir, not the shared common dir.
func TestWorktreeStateIsolation(t *testing.T) {
	main := gittest.NewTempRepo(t)

	worktreeDir := filepath.Join(t.TempDir(), "wt")
	main.Git(t, "worktree", "add", "-b", "wt-branch", worktreeDir)

	gitCommonDir := strings.TrimSpace(
		runGitIn(t, worktreeDir, "rev-parse", "--path-format=absolute", "--git-common-dir"),
	)
	worktreeGitDir := strings.TrimSpace(
		runGitIn(t, worktreeDir, "rev-parse", "--path-format=absolute", "--git-dir"),
	)
	require.NotEqual(t, gitCommonDir, worktreeGitDir)

	repo, err := git.OpenRepo(worktreeDir, gitCommonDir, worktreeGitDir)
	require.NoError(t, err)

	require.False(t, repo.IsRebaseInProgress())

	// Simulate a git-managed in-progress rebase by creating the worktree's
	// rebase-merge dir directly.
	require.NoError(t, os.MkdirAll(filepath.Join(worktreeGitDir, "rebase-merge"), 0o755))
	require.True(t, repo.IsRebaseInProgress())

	var payload struct{ Msg string }
	payload.Msg = "hello-worktree"
	require.NoError(t, repo.WriteStateFile(git.StateFileKindSyncV2, &payload))

	expected := filepath.Join(worktreeGitDir, "av", string(git.StateFileKindSyncV2))
	_, err = os.Stat(expected)
	require.NoError(t, err)

	commonPath := filepath.Join(gitCommonDir, "av", string(git.StateFileKindSyncV2))
	_, err = os.Stat(commonPath)
	require.True(t, os.IsNotExist(err))

	var loaded struct{ Msg string }
	require.NoError(t, repo.ReadStateFile(git.StateFileKindSyncV2, &loaded))
	require.Equal(t, "hello-worktree", loaded.Msg)

	require.NoError(t, repo.WriteStateFile(git.StateFileKindSyncV2, nil))
	_, err = os.Stat(expected)
	require.True(t, os.IsNotExist(err))
}

// TestLinkedWorktreeIgnoresCommonState ensures a linked worktree never reads
// or deletes state living in the shared common dir — that path is the main
// worktree's private state, not a legacy file to fall back to. Resolving it
// here would let one worktree clobber another's in-progress sync.
func TestLinkedWorktreeIgnoresCommonState(t *testing.T) {
	main := gittest.NewTempRepo(t)
	worktreeDir := filepath.Join(t.TempDir(), "wt")
	main.Git(t, "worktree", "add", "-b", "wt-isolated", worktreeDir)

	gitCommonDir := strings.TrimSpace(
		runGitIn(t, worktreeDir, "rev-parse", "--path-format=absolute", "--git-common-dir"),
	)
	worktreeGitDir := strings.TrimSpace(
		runGitIn(t, worktreeDir, "rev-parse", "--path-format=absolute", "--git-dir"),
	)
	repo, err := git.OpenRepo(worktreeDir, gitCommonDir, worktreeGitDir)
	require.NoError(t, err)

	// Stand in for the main worktree's live state.
	common := filepath.Join(gitCommonDir, "av", string(git.StateFileKindSyncV2))
	require.NoError(t, os.MkdirAll(filepath.Dir(common), 0o755))
	require.NoError(t, os.WriteFile(common, []byte(`{"Msg":"main"}`), 0o644))

	// The linked worktree has no state of its own: a read must miss, not
	// resolve to the common dir.
	var loaded struct{ Msg string }
	err = repo.ReadStateFile(git.StateFileKindSyncV2, &loaded)
	require.True(t, os.IsNotExist(err))

	// Clearing the linked worktree's (absent) state must leave the common
	// file untouched.
	require.NoError(t, repo.WriteStateFile(git.StateFileKindSyncV2, nil))
	_, err = os.Stat(common)
	require.NoError(t, err)
}

func runGitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	return string(out)
}

func TestGetRemoteName(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	avGitRepo := repo.AsAvGitRepo()
	require.Equal(t, avGitRepo.GetRemoteName(), git.DEFAULT_REMOTE_NAME)

	// This is a global config, so changing it here affects other tests. Be
	// sure to reset it.
	config.Av.Remote = "new-remote"
	require.Equal(t, avGitRepo.GetRemoteName(), "new-remote")
	config.Av.Remote = ""
}
