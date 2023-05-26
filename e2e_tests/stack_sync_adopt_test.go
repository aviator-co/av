package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStackSyncAdopt(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.Dir())

	require.Equal(t, 0, Cmd(t, "git", "checkout", "-b", "stack-1").ExitCode)
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n"), gittest.WithMessage("Commit 1a"))

	require.Equal(
		t,
		0,
		Av(t, "stack", "sync", "--no-fetch", "--no-push", "--parent", "main").ExitCode,
	)

	assert.Equal(t,
		meta.BranchState{
			Name:  "main",
			Trunk: true,
		},
		GetStoredParentBranchState(t, repo, "stack-1"),
		"stack-1 should be re-rooted onto main",
	)
}
