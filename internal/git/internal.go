package git

import (
	"os"
	"path/filepath"
)

// readGitFile reads a file from the per-worktree git directory (where git
// stores worktree-specific state like CHERRY_PICK_HEAD). In an additional
// worktree this differs from the shared common dir.
func (r *Repo) readGitFile(name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(r.WorktreeGitDir(), name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
