package git

import (
	"encoding/json"
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

func (r *Repo) stateFilePath(kind StateFileKind) string {
	return filepath.Join(r.WorktreeAvDir(), string(kind))
}

func (r *Repo) ReadStateFile(kind StateFileKind, msg any) error {
	bs, err := os.ReadFile(r.stateFilePath(kind))
	if err != nil {
		// Fall back to the legacy shared-AvDir path so in-flight syncs from
		// an older av version remain resumable after upgrade.
		if os.IsNotExist(err) {
			legacy := filepath.Join(r.AvDir(), string(kind))
			if legacy != r.stateFilePath(kind) {
				bs, err = os.ReadFile(legacy)
				if err != nil {
					return err
				}
				return json.Unmarshal(bs, msg)
			}
		}
		return err
	}
	return json.Unmarshal(bs, msg)
}

func (r *Repo) WriteStateFile(kind StateFileKind, msg any) error {
	if msg == nil {
		// Clear both the new and legacy locations. In a non-worktree repo
		// these resolve to the same path; only remove once.
		worktreePath := r.stateFilePath(kind)
		if err := os.Remove(worktreePath); err != nil && !os.IsNotExist(err) {
			return err
		}
		legacyPath := filepath.Join(r.AvDir(), string(kind))
		if legacyPath != worktreePath {
			if err := os.Remove(legacyPath); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	}

	bs, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(r.WorktreeAvDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(r.stateFilePath(kind), bs, 0o644)
}
