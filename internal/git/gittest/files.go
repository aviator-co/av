package gittest

import (
	"os"
	"path/filepath"
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
	fp := filepath.Join(repo.Dir(), filename)
	err := os.WriteFile(fp, body, 0644)
	require.NoError(t, err, "failed to write file: %s", filename)
	return fp
}

func AddFile(
	t *testing.T,
	repo *git.Repo,
	fp string,
) {
	_, err := repo.Git("add", fp)
	require.NoError(t, err, "failed to add file: %s", fp)
}
