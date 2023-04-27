package main

import (
	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/sirupsen/logrus"
	"os/exec"
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
	db, err := jsonfiledb.OpenRepo(repo)
	if err != nil {
		return nil, err
	}
	if len(db.ReadTx().AllBranches()) == 0 {
		logrus.Error("TODO: need to import existing ref metadata into database")
	}
	return db, nil
}
