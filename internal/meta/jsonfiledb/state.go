package jsonfiledb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/maputils"
)

func readState(filepath string) (*state, error) {
	data, err := os.ReadFile(filepath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if len(data) == 0 {
		data = []byte("{}")
	}
	var state state
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, errors.WrapIff(err, "failed to read av state file %q", filepath)
	}
	return &state, nil
}

type state struct {
	BranchState     map[string]meta.Branch `json:"branches"`
	RepositoryState meta.Repository        `json:"repository"`
}

func (d *state) copy() state {
	return state{
		maputils.Copy(d.BranchState),
		d.RepositoryState,
	}
}

// resolveStatePath follows symlinks in fp, so that a commit writes and renames
// next to the real av.db and keeps the symlink. If fp does not exist yet, it
// is returned unchanged and the commit creates it.
func resolveStatePath(fp string) string {
	if real, err := filepath.EvalSymlinks(fp); err == nil {
		return real
	}
	return fp
}

// write replaces the state file atomically: it writes a temporary file in the
// same directory and renames it over the old file, so a reader never sees a
// truncated or half-written av.db. The caller holds the av.db file lock, so
// the temporary file name does not need to be unique. The new file keeps the
// permissions of the old one. If there is no old file, it gets 0644 minus the
// umask, as before. The data and the rename are synced to disk before write
// returns.
func (d *state) write(fp string) error {
	perm := os.FileMode(0o644)
	fi, statErr := os.Stat(fp)
	if statErr == nil {
		perm = fi.Mode().Perm()
	}
	tmp := fp + ".tmp"
	_ = os.Remove(tmp)
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return errors.WrapIff(err, "failed to write av state file")
	}
	fail := func(err error) error {
		_ = f.Close()
		_ = os.Remove(tmp)
		return errors.WrapIff(err, "failed to write av state file")
	}
	if statErr == nil {
		// The umask applies to OpenFile. Set the old mode exactly.
		if err := f.Chmod(perm); err != nil {
			return fail(err)
		}
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(d); err != nil {
		return fail(err)
	}
	if err := f.Sync(); err != nil {
		return fail(err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return errors.WrapIff(err, "failed to write av state file")
	}
	if err := os.Rename(tmp, fp); err != nil {
		_ = os.Remove(tmp)
		return errors.WrapIff(err, "failed to write av state file")
	}
	if err := syncDir(filepath.Dir(fp)); err != nil {
		return errors.WrapIff(err, "failed to write av state file")
	}
	return nil
}

// syncDir syncs a directory so that a rename in it is on disk. Windows does
// not support this, and NTFS journals the rename.
func syncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	err = f.Sync()
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
