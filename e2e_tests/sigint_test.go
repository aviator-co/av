package e2e_tests

// TestAdoptExitsOnSIGINT lives outside the testscript harness because
// testscript v1.10.0 has no public API for sending arbitrary signals to a
// specifically named background process - it only sends os.Interrupt to
// every backgrounded command at end-of-test, which is not granular enough
// to verify that `av adopt` returns promptly after a single SIGINT while
// the merge-base step is still running.
//
// Once the project upgrades to a testscript version with a public
// signal-by-name primitive (or grows a custom command for it in
// testscript_test.go), this can move into testdata/script as a .txtar.

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
)

func TestAdoptExitsOnSIGINT(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("requires /bin/sh shim for the fake git binary")
	}

	repo := gittest.NewTempRepo(t)

	repo.Git(t, "checkout", "-b", "stack-1")
	repo.CommitFile(t, "my-file", "1a\n", gittest.WithMessage("Commit 1a"))
	repo.Git(t, "switch", "main")

	realGit, err := exec.LookPath("git")
	require.NoError(t, err)

	fakeGitDir := t.TempDir()
	markerPath := filepath.Join(t.TempDir(), "merge-base-started")
	fakeGitPath := filepath.Join(fakeGitDir, "git")
	fakeGit := `#!/bin/sh
if [ "$1" = "merge-base" ]; then
	if [ -n "$AV_TEST_GIT_MERGE_BASE_STARTED" ]; then
		: > "$AV_TEST_GIT_MERGE_BASE_STARTED"
	fi
	exec sleep 30
fi
exec "$REAL_GIT" "$@"
`
	require.NoError(t, os.WriteFile(fakeGitPath, []byte(fakeGit), 0o755))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cmd := exec.CommandContext(ctx, avCmdPath, "--debug", "adopt", "--dry-run")
	cmd.Dir = repo.RepoDir
	cmd.Env = append(
		os.Environ(),
		"PATH="+fakeGitDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"REAL_GIT="+realGit,
		"AV_TEST_GIT_MERGE_BASE_STARTED="+markerPath,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	require.NoError(t, cmd.Start())
	waitForFile(t, markerPath, 2*time.Second)

	require.NoError(t, cmd.Process.Signal(os.Interrupt))
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		// av returned within the deadline; that's the success path. The exit
		// code is non-zero because the run was cancelled, which is fine.
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("av adopt did not exit within 5s of SIGINT\nstdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}
}

func waitForFile(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("file %s did not appear within %s", path, timeout)
}
