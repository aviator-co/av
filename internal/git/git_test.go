package git_test

import (
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

func TestOpenRepoAllowsWorktreeConfigExtension(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	repo.Git(t, "config", "extensions.worktreeConfig", "true")

	_, err := git.OpenRepo(repo.RepoDir, repo.GitDir, repo.GitDir)
	require.NoError(t, err)
}

func TestOpenRepoAllowsWorktreeConfigExtensionInLinkedWorktree(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	repo.Git(t, "config", "extensions.worktreeConfig", "true")

	worktreeDir := filepath.Join(t.TempDir(), "linked")
	repo.Git(t, "worktree", "add", worktreeDir, "-b", "linked")

	linkedGitDir, linkedCommonGitDir := gitDirs(t, worktreeDir)
	_, err := git.OpenRepo(worktreeDir, linkedGitDir, linkedCommonGitDir)
	require.NoError(t, err)
}

func gitDirs(t *testing.T, dir string) (string, string) {
	t.Helper()

	cmd := exec.CommandContext(
		t.Context(),
		"git",
		"rev-parse",
		"--path-format=absolute",
		"--git-dir",
		"--git-common-dir",
	)
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	require.Len(t, lines, 2)
	return lines[0], lines[1]
}
