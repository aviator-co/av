package refmeta

import (
	"encoding/json"
	"os"
	"path/filepath"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
)

// ReadRepository reads repository metadata from the git repo.
// Returns the metadata and a boolean indicating if the metadata was found.
func ReadRepository(repo *git.Repo) (meta.Repository, error) {
	metaPath := filepath.Join(repo.Dir(), ".git", "av", "repo-metadata.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return meta.Repository{}, errors.WrapIf(err, "failed to read the repository metadata")
	}
	var repository meta.Repository
	if err := json.Unmarshal(data, &repository); err != nil {
		return meta.Repository{}, errors.WrapIf(err, "failed to unmarshal the repository metadata")
	}
	return repository, nil
}
