package git_test

import (
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRepo_ListRefs(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	refs, err := repo.ListRefs(&git.ListRefs{
		Patterns: []string{"refs/heads/*"},
	})
	require.NoError(t, err)
	require.Len(t, refs, 1, "expected exactly one ref (main)")

	main := refs[0]
	assert.Equal(t, "refs/heads/main", main.Name)
	assert.Equal(t, "commit", main.Type)
	assert.NotEmpty(t, main.Oid)
	assert.Empty(t, main.Upstream)
	assert.Empty(t, main.UpstreamStatus)
}
