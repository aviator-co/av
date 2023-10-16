package main

import (
	"os"
	"os/exec"
	"path"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/aviator-co/av/internal/meta/refmeta"
	"github.com/sirupsen/logrus"
)

var cachedRepo *git.Repo

func getRepo() (*git.Repo, error) {
	if cachedRepo == nil {
		cmd := exec.Command(
			"git",
			"rev-parse",
			"--path-format=absolute",
			"--show-toplevel",
			"--git-common-dir",
		)

		if rootFlags.Directory != "" {
			cmd.Dir = rootFlags.Directory
		}
		paths, err := cmd.Output()
		if err != nil {
			return nil, errors.Wrap(
				err,
				"failed to find git directory (are you running inside a Git repo?)",
			)
		}

		dir, gitDir, found := strings.Cut(strings.TrimSpace(string(paths)), "\n")
		if !found {
			return nil, errors.New("Unexpected format, not able to parse toplevel and common dir.")
		}

		cachedRepo, err = git.OpenRepo(dir, gitDir)
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
