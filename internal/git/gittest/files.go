package gittest

import (
	"os"
	"path"
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/stretchr/testify/require"
)

func CreateFile(
	t *testing.T,
	repo *git.Repo,
	filename string,
	body []byte,
) string {
	filepath := path.Join(repo.Dir(), filename)
	err := os.WriteFile(filepath, body, 0644)
	require.NoError(t, err, "failed to write file: %s", filename)
	return filepath
}

func AddFile(
	t *testing.T,
	repo *git.Repo,
	filepath string,
) {
	_, err := repo.Git("add", filepath)
	require.NoError(t, err, "failed to add file: %s", filepath)
}
