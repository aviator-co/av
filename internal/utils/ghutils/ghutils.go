package ghutils

import (
	"os"
	"path/filepath"

	"github.com/aviator-co/av/internal/git"
)

func HasCodeowners(repo *git.Repo) bool {
	if stat, _ := os.Stat(filepath.Join(repo.Dir(), ".github/CODEOWNERS")); stat != nil {
		return true
	}
	return false
}
