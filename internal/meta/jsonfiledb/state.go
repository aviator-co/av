package jsonfiledb

import (
	"encoding/json"
	"os"
	"path/filepath"

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

// write replaces the state file atomically: it writes a temporary file in the
// same directory and renames it over the old file, so a reader never sees a
// truncated or half-written av.db.
func (d *state) write(fp string) error {
	f, err := os.CreateTemp(filepath.Dir(fp), filepath.Base(fp)+".*.tmp")
	if err != nil {
		return errors.WrapIff(err, "failed to write av state file")
	}
	tmp := f.Name()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(d); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return errors.WrapIff(err, "failed to write av state file")
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return errors.WrapIff(err, "failed to write av state file")
	}
	if err := os.Chmod(tmp, 0o644); err != nil {
		_ = os.Remove(tmp)
		return errors.WrapIff(err, "failed to write av state file")
	}
	if err := os.Rename(tmp, fp); err != nil {
		_ = os.Remove(tmp)
		return errors.WrapIff(err, "failed to write av state file")
	}
	return nil
}
