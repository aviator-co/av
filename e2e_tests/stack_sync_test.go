package e2e_tests

import (
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestHelp(t *testing.T) {
	res := Av(t, "--help")
	assert.Equal(t, 0, res.ExitCode)
}

func TestStackSync(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.Dir())

	require.Equal(t, 0, Av(t, "stack", "branch", "stack-1").ExitCode)
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n"))
	require.Equal(t, 0, Av(t, "stack", "branch", "stack-2").ExitCode)
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n2a\n"))
	require.Equal(t, 0, Av(t, "stack", "sync", "--no-push").ExitCode)

	gittest.WithCheckoutBranch(t, repo, "stack-1", func() {
		gittest.CommitFile(t, repo, "my-file", []byte("1a\n1b\n"))
	})

	syncConflict := Av(t, "stack", "sync", "--no-push")
	assert.NotEqual(
		t, 0, syncConflict.ExitCode,
		"stack sync should return non-zero exit code if conflicts",
	)
	assert.Contains(t, syncConflict.Stderr, "conflict detected")
	assert.Contains(
		t, syncConflict.Stderr, "av stack sync --continue",
		"stack sync should print a message with instructions to continue",
	)

	syncContinueWithoutResolving := Av(t, "stack", "sync", "--continue")
	assert.NotEqual(
		t, 0, syncContinueWithoutResolving.ExitCode,
		"stack sync --continue should return non-zero exit code if conflicts have not been resolved",
	)
	// This message comes from Git, so it might be a touch brittle.
	// We'll see if it ever becomes an issue.
	assert.Contains(t, syncContinueWithoutResolving.Stdout, "fatal: Exiting because of an unresolved conflict.")

	// resolve the conflict
	err := ioutil.WriteFile(filepath.Join(repo.Dir(), "my-file"), []byte("1a\n1b\n2a\n"), 0644)
	require.NoError(t, err)
	_, err = repo.Git("add", "my-file")
	require.NoError(t, err, "failed to stage file")

	syncContinue := Av(t, "stack", "sync", "--continue")
	assert.Equal(t, 0, syncContinue.ExitCode, "stack sync --continue should return zero exit code after resolving conflicts")
}
