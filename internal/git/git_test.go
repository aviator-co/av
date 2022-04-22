package git_test

import (
	"github.com/aviator-co/av/internal/git"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os/exec"
	"testing"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

// Initialize a new git repository.
func initTempRepo(t *testing.T) *git.Repo {
	dir := t.TempDir()
	init := exec.Command("git", "init", "--initial-branch=main")
	init.Dir = dir

	err := init.Run()
	require.NoError(t, err, "failed to initialize git repository")

	repo, err := git.OpenRepo(dir)
	require.NoError(t, err, "failed to open repo")

	err = ioutil.WriteFile(dir+"/README.md", []byte("# Hello World"), 0644)
	require.NoError(t, err, "failed to write README.md")

	_, err = repo.Git("add", "README.md")
	require.NoError(t, err, "failed to stage README.md")

	_, err = repo.Git("commit", "-m", "Initial commit")
	require.NoError(t, err, "failed to create initial commit")

	return repo
}
