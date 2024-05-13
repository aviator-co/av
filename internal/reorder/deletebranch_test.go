package reorder

import (
	"bytes"
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/go-git/go-git/v5/plumbing"
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
	db := repo.OpenDB(t)
	out := &bytes.Buffer{}
	ctx := &Context{repo.AsAvGitRepo(), db, &State{Branch: "main"}, out}

	start := repo.GetCommitAtRef(t, plumbing.HEAD)

	repo.Git(t, "switch", "-c", "my-branch")
	repo.CommitFile(t, "file", "hello\n")

	// Need to switch back to main so that the branch ref can be deleted.
	// This shouldn't be an issue in the actual reorder because we'll never be
	// checked out on branches that we're about to delete (unless the user does
	// weird things™).
	repo.Git(t, "switch", "main")

	// delete-branch without --delete-git-ref should preserve the branch ref in Git
	err := DeleteBranchCmd{Name: "my-branch"}.Execute(ctx)
	require.NoError(t, err)
	repo.GetCommitAtRef(t, plumbing.NewBranchReferenceName("my-branch"))

	// delete-branch with --delete-git-ref should delete the branch ref in Git
	err = DeleteBranchCmd{Name: "my-branch", DeleteGitRef: true}.Execute(ctx)
	require.NoError(t, err)
	_, err = repo.GoGit.Reference(plumbing.NewBranchReferenceName("my-branch"), false)
	require.Error(t, err, "DeleteBranchCmd.Execute should delete the branch ref in Git")

	// subsequent delete-branch should be a no-op
	err = DeleteBranchCmd{Name: "my-branch", DeleteGitRef: true}.Execute(ctx)
	require.NoError(
		t,
		err,
		"DeleteBranchCmd.Execute should be a no-op if the branch ref is already deleted",
	)

	// HEAD of original branch should be preserved
	head := repo.GetCommitAtRef(t, plumbing.HEAD)
	require.Equal(
		t,
		start,
		head,
		"DeleteBranchCmd.Execute should preserve the HEAD of the original branch",
	)
}
