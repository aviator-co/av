package reorder

import (
	"bytes"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPickCmd_String(t *testing.T) {
	assert.Equal(t, "pick mycommit", PickCmd{Commit: "mycommit"}.String())
}

func TestPickCmd_Execute(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	out := &bytes.Buffer{}
	ctx := &Context{repo, State{Branch: "main"}, out}

	start, err := repo.RevParse(&git.RevParse{Rev: "HEAD"})
	require.NoError(t, err)
	next := gittest.CommitFile(t, repo, "file", []byte("hello\n"))

	t.Run("fast-forward commit", func(t *testing.T) {
		_, err = repo.Git("reset", "--hard", start)
		require.NoError(t, err)
		require.NoError(t, PickCmd{Commit: next}.Execute(ctx), "PickCmd.Execute should succeed with a fast-forward")
		require.Equal(t, next, ctx.State.Head, "PickCmd.Execute should update the state's head")
	})

	t.Run("conflicting commit", func(t *testing.T) {
		out.Reset()
		_, err = repo.Git("reset", "--hard", start)
		require.NoError(t, err)
		gittest.CommitFile(t, repo, "file", []byte("bonjour\n"))
		// Trying to re-apply the commit `next` should be a conflict
		err := PickCmd{Commit: next}.Execute(ctx)
		require.ErrorIs(t, err, ErrInterruptReorder, "PickCmd.Execute should return ErrInterruptReorder on conflict")
		require.Contains(t, out.String(), git.ShortSha(next), "PickCmd.Execute should print the conflicting commit")
	})
}
