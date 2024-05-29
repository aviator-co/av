package git

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type StateFileKind string

const (
	StateFileKindSync     StateFileKind = "stack-sync.state.json"
	StateFileKindReorder  StateFileKind = "stack-reorder.state.json"
	StateFileKindRestack  StateFileKind = "stack-restack.state.json"
	StateFileKindReparent StateFileKind = "stack-reparent.state.json"
)

func (r *Repo) ReadStateFile(kind StateFileKind, msg any) error {
	bs, err := os.ReadFile(filepath.Join(r.AvDir(), string(kind)))
	if err != nil {
		return err
	}
	return json.Unmarshal(bs, msg)
}

func (r *Repo) WriteStateFile(kind StateFileKind, msg any) error {
	if msg == nil {
		if err := os.Remove(filepath.Join(r.AvDir(), string(kind))); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	bs, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.AvDir(), string(kind)), bs, 0644)
}
