package git_test

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepo_Blame(t *testing.T) {
	repo := gittest.NewTempRepo(t)

	// Create initial commit with a file containing two lines.
	c1 := repo.CommitFile(t, "hello.txt", "line1\nline2\n", gittest.WithMessage("first commit"))

	// Append a third line in a second commit.
	c2 := repo.CommitFile(t, "hello.txt", "line1\nline2\nline3\n", gittest.WithMessage("second commit"))

	avRepo := repo.AsAvGitRepo()

	blame, err := avRepo.Blame(t.Context(), "hello.txt", "HEAD")
	require.NoError(t, err)

	// There should be three blame lines.
	require.Len(t, blame, 3)

	// Lines 1 and 2 were introduced by c1; line 3 by c2.
	assert.Equal(t, c1.String(), blame[0].CommitHash, "line 1 should be attributed to c1")
	assert.Equal(t, 1, blame[0].LineNo)

	assert.Equal(t, c1.String(), blame[1].CommitHash, "line 2 should be attributed to c1")
	assert.Equal(t, 2, blame[1].LineNo)

	assert.Equal(t, c2.String(), blame[2].CommitHash, "line 3 should be attributed to c2")
	assert.Equal(t, 3, blame[2].LineNo)
}

func TestRepo_Blame_NewFile(t *testing.T) {
	repo := gittest.NewTempRepo(t)

	// Commit a file on a branch so HEAD~1 does not have it.
	repo.CreateRef(t, plumbing.NewBranchReferenceName("feature"))
	repo.CheckoutBranch(t, plumbing.NewBranchReferenceName("feature"))
	repo.CommitFile(t, "newfile.txt", "content\n", gittest.WithMessage("add new file"))

	avRepo := repo.AsAvGitRepo()

	// Blaming at parent commit (where the file doesn't exist) should return empty, not an error.
	blame, err := avRepo.Blame(t.Context(), "newfile.txt", "HEAD~1")
	require.NoError(t, err)
	assert.Empty(t, blame, "blaming a file that doesn't exist at the given revision should return empty slice")
}

func TestParsePorcelainBlame(t *testing.T) {
	// Test the internal parser with a representative porcelain blame output.
	// We use a real repo call for integration coverage and test the parser logic via Blame().
	repo := gittest.NewTempRepo(t)

	hash := repo.CommitFile(
		t,
		"file.txt",
		"alpha\nbeta\ngamma\n",
		gittest.WithMessage("single commit"),
	)

	avRepo := repo.AsAvGitRepo()
	blame, err := avRepo.Blame(t.Context(), "file.txt", "HEAD")
	require.NoError(t, err)
	require.Len(t, blame, 3)

	for i, bl := range blame {
		assert.Equal(t, hash.String(), bl.CommitHash, "line %d should be attributed to the single commit", i+1)
		assert.Equal(t, i+1, bl.LineNo, "line number should be 1-based")
	}
}

func TestRepo_Blame_SpecificRevision(t *testing.T) {
	repo := gittest.NewTempRepo(t)

	c1 := repo.CommitFile(t, "data.txt", "original\n", gittest.WithMessage("original"))
	_ = repo.CommitFile(t, "data.txt", "original\nmodified\n", gittest.WithMessage("modified"))

	avRepo := repo.AsAvGitRepo()

	// Blame at c1 should only see one line attributed to c1.
	blame, err := avRepo.Blame(t.Context(), "data.txt", c1.String())
	require.NoError(t, err)
	require.Len(t, blame, 1)
	assert.Equal(t, c1.String(), blame[0].CommitHash)
	assert.Equal(t, 1, blame[0].LineNo)
}

func TestRepo_Blame_CommitHashSet(t *testing.T) {
	repo := gittest.NewTempRepo(t)

	// Two commits, interleaved lines.
	c1 := repo.CommitFile(t, "interleaved.txt", "A\nB\n", gittest.WithMessage("c1"))
	c2 := repo.CommitFile(t, "interleaved.txt", "A\nX\nB\nY\n", gittest.WithMessage("c2"))

	avRepo := repo.AsAvGitRepo()
	blame, err := avRepo.Blame(t.Context(), "interleaved.txt", "HEAD")
	require.NoError(t, err)
	require.Len(t, blame, 4)

	// After c2, line 1 = A (c1), line 2 = X (c2), line 3 = B (c1), line 4 = Y (c2)
	hashes := make(map[string]bool)
	for _, bl := range blame {
		hashes[bl.CommitHash] = true
	}
	assert.True(t, hashes[c1.String()], "c1 hash should appear in blame output")
	assert.True(t, hashes[c2.String()], "c2 hash should appear in blame output")

	_ = c1
	_ = c2
}
