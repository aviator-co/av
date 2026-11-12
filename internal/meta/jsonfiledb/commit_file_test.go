package jsonfiledb_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/stretchr/testify/require"
)

// A commit replaces av.db with a new file. It must keep the permissions of
// the old file, so a private av.db does not become world-readable.
func TestCommitKeepsFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	fp := filepath.Join(t.TempDir(), "av.db")
	require.NoError(t, os.WriteFile(fp, []byte("{}"), 0o600))
	require.NoError(t, os.Chmod(fp, 0o600))

	db, _, err := jsonfiledb.OpenPath(fp)
	require.NoError(t, err)
	tx := db.WriteTx()
	tx.SetBranch(meta.Branch{Name: "foo"})
	require.NoError(t, tx.Commit())

	fi, err := os.Stat(fp)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
}

// A commit through an av.db symlink must write the target and keep the link.
func TestCommitKeepsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need extra privileges on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "real", "av.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
	require.NoError(t, os.WriteFile(target, []byte("{}"), 0o644))
	link := filepath.Join(dir, "av.db")
	require.NoError(t, os.Symlink(target, link))

	db, _, err := jsonfiledb.OpenPath(link)
	require.NoError(t, err)
	tx := db.WriteTx()
	tx.SetBranch(meta.Branch{Name: "foo"})
	require.NoError(t, tx.Commit())

	fi, err := os.Lstat(link)
	require.NoError(t, err)
	require.NotZero(t, fi.Mode()&os.ModeSymlink, "av.db must still be a symlink")

	other, _, err := jsonfiledb.OpenPath(target)
	require.NoError(t, err)
	_, ok := other.ReadTx().Branch("foo")
	require.True(t, ok, "the commit must reach the symlink target")
}

// If Commit fails, the deferred Abort must not unlock the mutex a second
// time (a fatal error), and the DB must stay usable.
func TestAbortAfterFailedCommit(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "av.db")
	db, _, err := jsonfiledb.OpenPath(fp)
	require.NoError(t, err)

	tx := db.WriteTx()
	tx.SetBranch(meta.Branch{Name: "foo"})
	require.NoError(t, os.WriteFile(fp, []byte("not json"), 0o644))
	require.Error(t, tx.Commit())
	tx.Abort()

	require.NoError(t, os.WriteFile(fp, []byte("{}"), 0o644))
	tx = db.WriteTx()
	tx.SetBranch(meta.Branch{Name: "bar"})
	require.NoError(t, tx.Commit())
}
