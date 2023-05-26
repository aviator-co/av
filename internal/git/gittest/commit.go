package gittest

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/stretchr/testify/require"
)

type commitFileOpts struct {
	msg   string
	amend bool
}

type CommitFileOpt func(*commitFileOpts)

func WithMessage(msg string) CommitFileOpt {
	return func(opts *commitFileOpts) {
		opts.msg = msg
	}
}

func WithAmend() CommitFileOpt {
	return func(opts *commitFileOpts) {
		opts.amend = true
	}
}

func CommitFile(
	t *testing.T,
	repo *git.Repo,
	filename string,
	body []byte,
	cfOpts ...CommitFileOpt,
) string {
	opts := commitFileOpts{
		msg: fmt.Sprintf("Write %s", filename),
	}
	for _, o := range cfOpts {
		o(&opts)
	}

	filepath := path.Join(repo.Dir(), filename)
	err := os.WriteFile(filepath, body, 0644)
	require.NoError(t, err, "failed to write file: %s", filename)

	_, err = repo.Git("add", filepath)
	require.NoError(t, err, "failed to add file: %s", filename)

	args := []string{"commit", "-m", opts.msg}
	if opts.amend {
		args = append(args, "--amend")
	}
	_, err = repo.Git(args...)
	require.NoError(t, err, "failed to commit file: %s", filename)

	head, err := repo.RevParse(&git.RevParse{Rev: "HEAD"})
	require.NoError(t, err, "failed to get HEAD")
	return head
}
