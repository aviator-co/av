package git_test

import (
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/assert"
)

func TestRepo_Log(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	c1 := gittest.CommitFile(t, repo, "file", []byte("first commit\n"), gittest.WithMessage("commit 1\n\ncommit 1 body"))
	c2 := gittest.CommitFile(t, repo, "file", []byte("first commit\nsecond commit\n"), gittest.WithMessage("commit 2\n\ncommit 2 body"))

	cis, err := repo.Log(git.LogOpts{RevisionRange: []string{c2, "^" + c1 + "^1"}})
	assert.NoError(t, err)
	assert.Equal(t, []*git.CommitInfo{
		{
			Hash:      c2,
			ShortHash: c2[:7],
			Subject:   "commit 2",
			Body:      "commit 2 body\n",
		},
		{
			Hash:      c1,
			ShortHash: c1[:7],
			Subject:   "commit 1",
			Body:      "commit 1 body\n",
		},
	}, cis)
}

func TestFindClosedPRs(t *testing.T) {
	cis := []*git.CommitInfo{
		{
			Hash: "fake_1",
			Body: "some comments. close #123. fixed #433",
		},
		{
			Hash: "fake_2",
			Body: "some other comments.\nfix #234",
		},
	}

	assert.Equal(t, map[int64]string{
		123: "fake_1",
		234: "fake_2",
		433: "fake_1",
	}, git.FindClosesPullRequestComments(cis))
}
