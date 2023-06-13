package reorder

import (
	"bytes"
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteBranchCmd_String(t *testing.T) {
	assert.Equal(t, "delete-branch my-branch", DeleteBranchCmd{Name: "my-branch"}.String())
	assert.Equal(
		t,
		"delete-branch my-branch --delete-git-ref",
		DeleteBranchCmd{Name: "my-branch", DeleteGitRef: true}.String(),
	)
}

func TestDeleteBranchCmd_Execute(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	db, err := jsonfiledb.OpenRepo(repo)
	require.NoError(t, err)
	out := &bytes.Buffer{}
	ctx := &Context{repo, db, &State{Branch: "main"}, out}

	start, err := repo.RevParse(&git.RevParse{Rev: "HEAD"})
	require.NoError(t, err)

	_, err = repo.Git("switch", "-c", "my-branch")
	require.NoError(t, err)
	gittest.CommitFile(t, repo, "file", []byte("hello\n"))

	// Need to switch back to main so that the branch ref can be deleted.
	// This shouldn't be an issue in the actual reorder because we'll never be
	// checked out on branches that we're about to delete (unless the user does
	// weird things™).
	_, err = repo.Git("switch", "main")
	require.NoError(t, err)

	// delete-branch without --delete-git-ref should preserve the branch ref in Git
	err = DeleteBranchCmd{Name: "my-branch"}.Execute(ctx)
	require.NoError(t, err)
	_, err = repo.RevParse(&git.RevParse{Rev: "my-branch"})
	require.NoError(t, err, "DeleteBranchCmd.Execute should preserve the branch ref in Git")

	// delete-branch with --delete-git-ref should delete the branch ref in Git
	err = DeleteBranchCmd{Name: "my-branch", DeleteGitRef: true}.Execute(ctx)
	require.NoError(t, err)
	_, err = repo.RevParse(&git.RevParse{Rev: "my-branch"})
	require.Error(t, err, "DeleteBranchCmd.Execute should delete the branch ref in Git")

	// subsequent delete-branch should be a no-op
	err = DeleteBranchCmd{Name: "my-branch", DeleteGitRef: true}.Execute(ctx)
	require.NoError(
		t,
		err,
		"DeleteBranchCmd.Execute should be a no-op if the branch ref is already deleted",
	)

	// HEAD of original branch should be preserved
	head, err := repo.RevParse(&git.RevParse{Rev: "HEAD"})
	require.NoError(t, err)
	require.Equal(
		t,
		start,
		head,
		"DeleteBranchCmd.Execute should preserve the HEAD of the original branch",
	)
}
