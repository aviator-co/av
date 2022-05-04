package gittest

import (
	"fmt"
	"github.com/aviator-co/av/internal/git"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"path"
	"testing"
)

func CommitFile(t *testing.T, repo *git.Repo, filename string, body []byte) {
	filepath := path.Join(repo.Dir(), filename)
	err := ioutil.WriteFile(filepath, body, 0644)
	require.NoError(t, err, "failed to write file: %s", filename)

	_, err = repo.Git("add", filepath)
	require.NoError(t, err, "failed to add file: %s", filename)

	msg := fmt.Sprintf("write file %s", filename)
	_, err = repo.Git("commit", "-m", msg)
	require.NoError(t, err, "failed to commit file: %s", filename)
}
