package reorder

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPickCmd_String(t *testing.T) {
	assert.Equal(t, "pick mycommit", PickCmd{Commit: "mycommit"}.String())
	assert.Equal(t, "squash mycommit", PickCmd{Commit: "mycommit", Mode: PickModeSquash}.String())
	assert.Equal(t, "fixup mycommit", PickCmd{Commit: "mycommit", Mode: PickModeFixup}.String())
	assert.Equal(t, "squash mycommit  # a comment", PickCmd{Commit: "mycommit", Comment: "a comment", Mode: PickModeSquash}.String())
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
		require.Equal(
			t,
			next.String(),
			ctx.State.Head,
			"PickCmd.Execute should update the state's head",
		)
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

	t.Run("fixup mode folds commit and preserves first message", func(t *testing.T) {
		out.Reset()
		repo.Git(t, "reset", "--hard", start.String())
		// Commit A: adds a.txt (the "previous" commit that we squash into)
		_ = repo.CommitFile(t, "a.txt", "aaa\n", gittest.WithMessage("first commit message"))
		// Commit B (the one to fixup-pick into A): adds b.txt
		commitB := repo.CommitFile(t, "b.txt", "bbb\n", gittest.WithMessage("second commit message"))

		// Reset back to commit A so we can replay with Execute in fixup mode.
		commitAHash := strings.TrimSpace(repo.Git(t, "rev-parse", "HEAD~1"))
		repo.Git(t, "reset", "--hard", commitAHash)

		// Now HEAD is at commit A; re-apply commitB as fixup.
		// After fixup, commitA and commitB are merged into a single amended commit.
		fixCtx := &Context{repo.AsAvGitRepo(), db, &State{Branch: "main"}, out}
		err := PickCmd{Commit: commitB.String(), Mode: PickModeFixup}.Execute(fixCtx)
		require.NoError(t, err, "fixup Execute should succeed")

		// After the fixup squash, the two commits (commitA + cherry-picked commitB)
		// have been folded into one: HEAD~1 should now be `start` (the initial commit).
		parentHash := strings.TrimSpace(repo.Git(t, "rev-parse", "HEAD~1"))
		assert.Equal(t, start.String(), parentHash, "HEAD~1 should be the initial commit — the two commits were squashed into one")

		// The message should be the first commit's message (fixup discards the second).
		headMsg := strings.TrimSpace(repo.Git(t, "log", "-1", "--format=%B", "HEAD"))
		assert.Equal(t, "first commit message", headMsg)

		// Both files must exist in the working tree.
		_, errA := os.Stat(filepath.Join(repo.RepoDir, "a.txt"))
		assert.NoError(t, errA, "a.txt should exist after fixup")
		_, errB := os.Stat(filepath.Join(repo.RepoDir, "b.txt"))
		assert.NoError(t, errB, "b.txt should exist after fixup")
	})
}

// TestPerformSquash_Fixup verifies that PerformSquash with PickModeFixup folds
// HEAD into HEAD~1, keeps HEAD~1's message, and the working tree contains both
// files from both commits.
func TestPerformSquash_Fixup(t *testing.T) {
	repo := gittest.NewTempRepo(t)

	// Create first commit: adds a.txt
	_ = repo.CommitFile(t, "a.txt", "aaa\n", gittest.WithMessage("first commit"))
	firstCommitHash := strings.TrimSpace(repo.Git(t, "rev-parse", "HEAD"))

	// Create second commit: adds b.txt
	headHash := repo.CommitFile(t, "b.txt", "bbb\n", gittest.WithMessage("second commit"))

	cmd := PickCmd{Commit: headHash.String(), Mode: PickModeFixup}
	err := cmd.PerformSquash(t.Context(), repo.AsAvGitRepo(), "")
	require.NoError(t, err, "PerformSquash with PickModeFixup should succeed")

	// After squash there should be exactly one commit on top of the initial repo commit.
	// HEAD~1 should be the commit before the first commit we created.
	newHead := strings.TrimSpace(repo.Git(t, "rev-parse", "HEAD"))
	assert.NotEqual(t, headHash.String(), newHead, "HEAD should have changed after fixup squash")

	// HEAD~1 should be the commit that was HEAD~2 before (the initial commit)
	parentOfFirst := strings.TrimSpace(repo.Git(t, "rev-parse", firstCommitHash+"~1"))
	newParent := strings.TrimSpace(repo.Git(t, "rev-parse", "HEAD~1"))
	assert.Equal(t, parentOfFirst, newParent, "HEAD~1 should be one commit (the two were squashed)")

	// Message should be first commit's message
	headMsg := strings.TrimSpace(repo.Git(t, "log", "-1", "--format=%B", "HEAD"))
	assert.Equal(t, "first commit", headMsg, "HEAD message should be the first commit's message after fixup")

	// Both files must be present
	_, errA := os.Stat(filepath.Join(repo.RepoDir, "a.txt"))
	assert.NoError(t, errA, "a.txt should exist after fixup squash")
	_, errB := os.Stat(filepath.Join(repo.RepoDir, "b.txt"))
	assert.NoError(t, errB, "b.txt should exist after fixup squash")
}

// TestPerformSquash_FirstCommit_ReturnsError verifies that PerformSquash fails
// gracefully when there is no parent commit to fold into.
func TestPerformSquash_FirstCommit_ReturnsError(t *testing.T) {
	repo := gittest.NewTempRepo(t)

	// The gittest.NewTempRepo already has one commit (initial). Detach HEAD to
	// an orphan by creating a new branch with no parents would require low-level
	// manipulation; instead we rely on the fact that HEAD~1 at the initial commit
	// has no parent accessible within the usual branch history. We create a
	// separate detached commit chain so that rev-parse HEAD~1 fails.
	//
	// We use a simpler approach: create a branch with only one commit where the
	// branch's first commit has a parent (the repo's initial commit). PerformSquash
	// does a `git rev-parse HEAD~1` and checks if it fails. So we need a state
	// where there is genuinely no reachable HEAD~1 from the standpoint of the
	// squash guard, but in a standard git repo that's hard to arrange without
	// detached HEAD and orphan commits.
	//
	// The guard in PerformSquash checks whether `repo.RevParse(HEAD~1)` returns an
	// error. This happens when HEAD is the very first commit in the repository
	// (an orphan commit). We'll create an orphan commit to simulate this.
	repo.Git(t, "checkout", "--orphan", "orphan-branch")
	repo.Git(t, "rm", "-rf", ".")
	_ = repo.CommitFile(t, "only.txt", "only\n", gittest.WithMessage("only commit"))

	// Now HEAD has no parent — HEAD~1 does not resolve.
	headHash := repo.GetCommitAtRef(t, plumbing.HEAD)
	cmd := PickCmd{Commit: headHash.String(), Mode: PickModeFixup}
	err := cmd.PerformSquash(t.Context(), repo.AsAvGitRepo(), "")
	require.Error(t, err, "PerformSquash should return an error when there is no parent commit")
	assert.Contains(t, err.Error(), "no previous commit", "error should mention there is no previous commit")
}
