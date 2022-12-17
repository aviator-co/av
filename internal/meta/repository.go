package meta

import (
	"emperror.dev/errors"
	"encoding/json"
	"github.com/aviator-co/av/internal/git"
	"github.com/sirupsen/logrus"
	"os"
	"path"
)

type Repository struct {
	// The GitHub (GraphQL) ID of the repository (e.g., R_kgDOHMmHmg).
	ID string `json:"id"`
	// The owner of the repository (e.g., aviator-co)
	Owner string `json:"owner"`
	// The name of the repository (e.g., av)
	Name string `json:"name"`
}

var ErrRepoNotInitialized = errors.New("this repository not initialized: please run `av init`")

// ReadRepository reads repository metadata from the git repo.
// Returns the metadata and a boolean indicating if the metadata was found.
func ReadRepository(repo *git.Repo) (Repository, error) {
	var meta Repository

	metaPath := path.Join(repo.Dir(), ".git", "av", "repo-metadata.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return meta, ErrRepoNotInitialized
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		logrus.WithError(err).Error("repository metadata file is corrupt - ignoring")
		return meta, ErrRepoNotInitialized
	}
	return meta, nil
}

// WriteRepository writes repository metadata to the git repo.
// It can be loaded again with ReadRepository.
func WriteRepository(repo *git.Repo, meta Repository) error {
	if err := os.Mkdir(path.Join(repo.Dir(), ".git", "av"), 0755); err != nil && !os.IsExist(err) {
		return errors.Wrap(err, "failed to create av metadata directory")
	}
	metaPath := path.Join(repo.Dir(), ".git", "av", "repo-metadata.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal repository metadata")
	}
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return errors.Wrap(err, "failed to write repository metadata")
	}
	return nil
}
