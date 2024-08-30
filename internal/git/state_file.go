package git

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type StateFileKind string

const (
	StateFileKindSync    StateFileKind = "stack-sync.state.json"
	StateFileKindReorder StateFileKind = "stack-reorder.state.json"
	StateFileKindRestack StateFileKind = "stack-restack.state.json"
	StateFileKindSyncV2  StateFileKind = "stack-sync-v2.state.json"
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
		if err := os.Remove(filepath.Join(r.AvDir(), string(kind))); err != nil &&
			!os.IsNotExist(err) {
			return err
		}
		return nil
	}

	file := filepath.Join(r.AvDir(), string(kind))
	if _, err := os.Stat(file); err == nil {
		// When the state file already exists, it means that during fixing conflicts
		return fmt.Errorf("Please execute the command after fixing the conflict")
	}

	bs, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, bs, 0644)
}
