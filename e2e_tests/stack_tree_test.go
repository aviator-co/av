package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
)

func TestStackTree(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.Dir())

	RequireAv(t, "stack", "branch", "foo")
	gittest.CommitFile(t, repo, "foo", []byte("foo"))

	RequireAv(t, "stack", "branch", "bar")
	gittest.CommitFile(t, repo, "bar", []byte("bar"))

	gittest.CheckoutBranch(t, repo, "main")
	RequireAv(t, "stack", "branch", "spam")
	gittest.CommitFile(t, repo, "spam", []byte("spam"))

	RequireAv(t, "stack", "tree")
}
