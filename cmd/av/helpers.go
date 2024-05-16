package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/aviator-co/av/internal/meta/refmeta"
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

var ErrRepoNotInitialized = errors.Sentinel(
	"this repository is not initialized; please run `av init`",
)

func getDB(repo *git.Repo) (meta.DB, error) {
	db, exists, err := getOrCreateDB(repo)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrRepoNotInitialized
	}
	return db, nil
}

func getOrCreateDB(repo *git.Repo) (meta.DB, bool, error) {
	dbPath := filepath.Join(repo.AvDir(), "av.db")
	oldDBPathPath := filepath.Join(repo.AvDir(), "repo-metadata.json")
	dbPathStat, _ := os.Stat(dbPath)
	oldDBPathStat, _ := os.Stat(oldDBPathPath)

	if dbPathStat == nil && oldDBPathStat != nil {
		// Migrate old db to new db
		db, exists, err := jsonfiledb.OpenPath(dbPath)
		if err != nil {
			return nil, false, err
		}
		if err := refmeta.Import(repo, db); err != nil {
			return nil, false, errors.WrapIff(err, "failed to import ref metadata into av database")
		}
		return db, exists, nil
	}
	return jsonfiledb.OpenPath(dbPath)
}
