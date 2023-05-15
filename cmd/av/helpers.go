package main

import (
	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/aviator-co/av/internal/meta/refmeta"
	"github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"path"
	"strings"
)

var cachedRepo *git.Repo

func getRepo() (*git.Repo, error) {
	if cachedRepo == nil {
		cmd := exec.Command("git", "rev-parse", "--show-toplevel")
		if rootFlags.Directory != "" {
			cmd.Dir = rootFlags.Directory
		}
		toplevel, err := cmd.Output()
		if err != nil {
			return nil, errors.Wrap(err, "failed to determine repo toplevel (are you running inside a Git repo?)")
		}
		cachedRepo, err = git.OpenRepo(strings.TrimSpace(string(toplevel)))
		if err != nil {
			return nil, errors.Wrap(err, "failed to open git repo")
		}
	}
	return cachedRepo, nil
}

func getDB(repo *git.Repo) (meta.DB, error) {
	dbPath := path.Join(repo.AvDir(), "av.db")
	existingStat, _ := os.Stat(dbPath)
	db, err := jsonfiledb.OpenPath(dbPath)
	if err != nil {
		return nil, err
	}
	if existingStat == nil {
		logrus.Debug("Initializing new av database")
		if err := refmeta.Import(repo, db); err != nil {
			return nil, errors.WrapIff(err, "failed to import ref metadata into av database")
		}
	}
	return db, nil
}
