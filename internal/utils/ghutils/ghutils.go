package ghutils

import (
	"os"
	"path"

	"github.com/aviator-co/av/internal/git"
)

func HasCodeowners(repo *git.Repo) bool {
	if stat, _ := os.Stat(path.Join(repo.Dir(), ".github/CODEOWNERS")); stat != nil {
		return true
	}
	return false
}
