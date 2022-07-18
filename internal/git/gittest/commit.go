package gittest

import (
	"fmt"
	"github.com/aviator-co/av/internal/git"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"path"
	"testing"
)

type commitFileOpts struct {
	msg string
}

type CommitFileOpt func(*commitFileOpts)

func WithMessage(msg string) CommitFileOpt {
	return func(opts *commitFileOpts) {
		opts.msg = msg
	}
}

func CommitFile(t *testing.T, repo *git.Repo, filename string, body []byte, os ...CommitFileOpt) {
	opts := commitFileOpts{
		msg: fmt.Sprintf("Write %s", filename),
	}
	for _, o := range os {
		o(&opts)
	}

	filepath := path.Join(repo.Dir(), filename)
	err := ioutil.WriteFile(filepath, body, 0644)
	require.NoError(t, err, "failed to write file: %s", filename)

	_, err = repo.Git("add", filepath)
	require.NoError(t, err, "failed to add file: %s", filename)

	_, err = repo.Git("commit", "-m", opts.msg)
	require.NoError(t, err, "failed to commit file: %s", filename)
}
