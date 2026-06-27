package ghutils

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/aviator-co/av/internal/git"
)

func HasCodeowners(repo *git.Repo) bool {
	_, err := os.Stat(filepath.Join(repo.Dir(), ".github/CODEOWNERS"))
	if err == nil {
		return true
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	// For permission/IO errors, assume the file exists to be safe.
	return true
}
