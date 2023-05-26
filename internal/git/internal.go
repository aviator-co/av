package git

import (
	"os"
	"path/filepath"
)

// readGitFile reads a file from the .git directory.
func (r *Repo) readGitFile(name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(r.GitDir(), name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
