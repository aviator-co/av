package meta

import (
	"emperror.dev/errors"
	"encoding/json"
	"github.com/aviator-co/av/internal/git"
	"github.com/sirupsen/logrus"
	"io/ioutil"
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

func GetRepository(repo *git.Repo) (Repository, bool) {
	var meta Repository

	metaPath := path.Join(repo.Dir(), ".git", "av", "repo-metadata.json")
	data, err := ioutil.ReadFile(metaPath)
	if err != nil {
		return meta, false
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		logrus.WithError(err).Error("repository metadata file is corrupt - ignoring")
		return meta, false
	}
	return meta, true
}

func WriteRepository(repo *git.Repo, meta Repository) error {
	if err := os.Mkdir(path.Join(repo.Dir(), ".git", "av"), 0755); err != nil && !os.IsExist(err) {
		return errors.Wrap(err, "failed to create av metadata directory")
	}
	metaPath := path.Join(repo.Dir(), ".git", "av", "repo-metadata.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal repository metadata")
	}
	if err := ioutil.WriteFile(metaPath, data, 0644); err != nil {
		return errors.Wrap(err, "failed to write repository metadata")
	}
	return nil
}
