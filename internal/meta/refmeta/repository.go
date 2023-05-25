package refmeta

import (
	"encoding/json"
	"os"
	"path"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/sirupsen/logrus"
)

var ErrRepoNotInitialized = errors.Sentinel("this repository not initialized: please run `av init`")

// ReadRepository reads repository metadata from the git repo.
// Returns the metadata and a boolean indicating if the metadata was found.
func ReadRepository(repo *git.Repo) (meta.Repository, error) {
	var repository meta.Repository

	metaPath := path.Join(repo.Dir(), ".git", "av", "repo-metadata.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return repository, ErrRepoNotInitialized
	}
	if err := json.Unmarshal(data, &repository); err != nil {
		logrus.WithError(err).Error("repository metadata file is corrupt - ignoring")
		return repository, ErrRepoNotInitialized
	}
	return repository, nil
}
