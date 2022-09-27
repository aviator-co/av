package ghutils

import (
	"github.com/aviator-co/av/internal/git"
	"os"
	"path"
)

func HasCodeowners(repo *git.Repo) bool {
	if stat, _ := os.Stat(path.Join(repo.Dir(), ".github/CODEOWNERS")); stat != nil {
		return true
	}
	return false
}
