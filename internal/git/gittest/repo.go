package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

// NewTempRepo initializes a new git repository with reasonable defaults.
func NewTempRepo(t *testing.T) *git.Repo {
	var dir string
	var remoteDir string
	if os.Getenv("AV_TEST_PRESERVE_TEMP_REPO") != "" {
		var err error
		dir, err = os.MkdirTemp("", "repo")
		require.NoError(t, err)
		logrus.Infof("created git test repo: %s", dir)

		remoteDir, err = os.MkdirTemp("", "remote-repo")
		require.NoError(t, err)
		logrus.Infof("created git remote test repo: %s", remoteDir)
	} else {
		dir = filepath.Join(t.TempDir(), "local")
		require.NoError(t, os.MkdirAll(dir, 0755))

		remoteDir = filepath.Join(t.TempDir(), "remote")
		require.NoError(t, os.MkdirAll(remoteDir, 0755))
	}
	init := exec.Command("git", "init", "--initial-branch=main")
	init.Dir = dir

	err := init.Run()
	require.NoError(t, err, "failed to initialize git repository")

	remoteInit := exec.Command("git", "init", "--bare")
	remoteInit.Dir = remoteDir

	err = remoteInit.Run()
	require.NoError(t, err, "failed to initialize remote git repository")

	repo, err := git.OpenRepo(dir)
	require.NoError(t, err, "failed to open repo")

	settings := map[string]string{
		"user.name":  "av-test",
		"user.email": "av-test@nonexistant",
	}
	for k, v := range settings {
		_, err = repo.Git("config", k, v)
		require.NoErrorf(t, err, "failed to set config %s=%s", k, v)
	}

	_, err = repo.Git("remote", "add", "origin", remoteDir, "--master=main")
	require.NoError(t, err, "failed to set remote")

	err = os.WriteFile(dir+"/README.md", []byte("# Hello World"), 0644)
	require.NoError(t, err, "failed to write README.md")

	_, err = repo.Git("add", "README.md")
	require.NoError(t, err, "failed to stage README.md")

	_, err = repo.Git("commit", "-m", "Initial commit")
	require.NoError(t, err, "failed to create initial commit")

	_, err = repo.Git("push", "origin", "main")
	require.NoError(t, err, "failed to push to remote")

	db, err := jsonfiledb.OpenRepo(repo)
	require.NoError(t, err, "failed to open database")

	// Write metadata because some commands expect it to be there.
	// This repository obviously doesn't exist so tests still need to be careful
	// not to invoke operations that would communicate with GitHub (e.g.,
	// by using the `--no-fetch` and `--no-push` flags).
	tx := db.WriteTx()
	tx.SetRepository(meta.Repository{
		ID:    "R_nonexistent_",
		Owner: "aviator-co",
		Name:  "nonexistent",
	})
	require.NoError(t, tx.Commit(), "failed to write repository metadata")

	return repo
}
