package e2e_tests

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/stretchr/testify/require"
)

func TestStackSyncAll(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.Dir())

	require.Equal(t, 0, Cmd(t, "git", "switch", "main").ExitCode)
	RequireAv(t, "stack", "branch", "stack-1")
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n"), gittest.WithMessage("Commit 1a"))

	require.Equal(t, 0, Cmd(t, "git", "switch", "main").ExitCode)
	RequireAv(t, "stack", "branch", "stack-2")
	gittest.CommitFile(t, repo, "my-file", []byte("2a\n"), gittest.WithMessage("Commit 2a"))

	require.Equal(t, 0, Cmd(t, "git", "switch", "main").ExitCode)
	gittest.CommitFile(t, repo, "other-file", []byte("X2\n"), gittest.WithMessage("Commit X2"))
	require.Equal(t, 0, Cmd(t, "git", "push", "origin", "main").ExitCode)

	//     main:    X  -> X2
	//     stack-1:  \ -> 1a
	//     stack-2:  \ -> 2a

	require.Equal(
		t,
		0,
		Av(t, "stack", "sync", "--no-fetch", "--no-push", "--trunk", "--all").ExitCode,
	)

	//     main:    X  -> X2
	//     stack-1:        \ -> 1a
	//     stack-2:        \ -> 2a

	require.Equal(t, 0,
		Cmd(t, "git", "merge-base", "--is-ancestor", "main", "stack-1").ExitCode,
		"HEAD of main should be an ancestor of HEAD of stack-1",
	)
	require.Equal(t, 0,
		Cmd(t, "git", "merge-base", "--is-ancestor", "main", "stack-2").ExitCode,
		"HEAD of main should be an ancestor of HEAD of stack-2",
	)
}
