package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
)

func TestStackTree(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	RequireAv(t, "stack", "branch", "foo")
	repo.CommitFile(t, "foo", "foo")

	RequireAv(t, "stack", "branch", "bar")
	repo.CommitFile(t, "bar", "bar")

	repo.CheckoutBranch(t, "refs/heads/main")
	RequireAv(t, "stack", "branch", "spam")
	repo.CommitFile(t, "spam", "spam")

	RequireAv(t, "stack", "tree")
}
