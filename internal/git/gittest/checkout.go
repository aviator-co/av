package gittest

import (
	"github.com/aviator-co/av/internal/git"
	"github.com/stretchr/testify/require"
	"testing"
)

// WithCheckoutBranch runs the given function after checking out the specified branch.
// It returns to the original branch after the function returns.
func WithCheckoutBranch(t *testing.T, repo *git.Repo, branch string, f func()) {
	original, err := repo.CheckoutBranch(&git.CheckoutBranch{Name: branch})
	require.NoError(t, err)
	defer func() {
		_, err := repo.CheckoutBranch(&git.CheckoutBranch{Name: original})
		require.NoError(t, err)
	}()
	f()
}
