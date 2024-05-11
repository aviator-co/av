package reorder

import (
	"bytes"
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPickCmd_String(t *testing.T) {
	assert.Equal(t, "pick mycommit", PickCmd{Commit: "mycommit"}.String())
}

func TestPickCmd_Execute(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	db := repo.OpenDB(t)
	out := &bytes.Buffer{}
	ctx := &Context{repo.AsAvGitRepo(), db, &State{Branch: "main"}, out}

	start := repo.GetCommitAtRef(t, plumbing.HEAD)
	next := repo.CommitFile(t, "file", "hello\n")

	t.Run("fast-forward commit", func(t *testing.T) {
		repo.Git(t, "reset", "--hard", start.String())
		require.NoError(
			t,
			PickCmd{Commit: next.String()}.Execute(ctx),
			"PickCmd.Execute should succeed with a fast-forward",
		)
		require.Equal(t, next.String(), ctx.State.Head, "PickCmd.Execute should update the state's head")
	})

	t.Run("conflicting commit", func(t *testing.T) {
		out.Reset()
		repo.Git(t, "reset", "--hard", start.String())
		repo.CommitFile(t, "file", "bonjour\n")
		// Trying to re-apply the commit `next` should be a conflict
		err := PickCmd{Commit: next.String()}.Execute(ctx)
		require.ErrorIs(
			t,
			err,
			ErrInterruptReorder,
			"PickCmd.Execute should return ErrInterruptReorder on conflict",
		)
		require.Contains(
			t,
			out.String(),
			git.ShortSha(next.String()),
			"PickCmd.Execute should print the conflicting commit",
		)
	})
}
