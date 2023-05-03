package jsonfiledb

import (
	"emperror.dev/errors"
	"encoding/json"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/maputils"
	"os"
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

func (d *state) write(filepath string) error {
	f, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return errors.WrapIff(err, "failed to write av state file")
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(d); err != nil {
		_ = f.Close()
		return errors.WrapIff(err, "failed to write av state file")
	}
	return f.Close()
}
