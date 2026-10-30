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

// ReadStateFile reads the per-worktree state file. Each worktree (including
// the main checkout) operates only on its own WorktreeAvDir(); we deliberately
// do NOT fall back to the shared common-dir path. In the main checkout
// WorktreeAvDir() *is* the common dir, so a pre-upgrade state file is read
// directly. In a linked worktree the common dir now holds the main worktree's
// private state — reading it here would resolve another worktree's live sync
// as our own and the orphan check would then delete it.
func (r *Repo) ReadStateFile(kind StateFileKind, msg any) error {
	bs, err := os.ReadFile(r.stateFilePath(kind))
	if err != nil {
		return err
	}
	return json.Unmarshal(bs, msg)
}

func (r *Repo) WriteStateFile(kind StateFileKind, msg any) error {
	if msg == nil {
		if err := os.Remove(r.stateFilePath(kind)); err != nil && !os.IsNotExist(err) {
			return err
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
