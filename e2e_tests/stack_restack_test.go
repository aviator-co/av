package e2e_tests

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"
)

func TestStackRestack(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	// To start, we create a simple three-stack where each stack has a single commit.
	// Our stack looks like:
	//     stack-1: main -> 1a
	//     stack-2:           \ -> 2a
	//     stack-3:           |    \ -> 3a
	//     stack-4:           \ -> 4a
	// Note: we create the first branch with a "vanilla" git checkout just to
	// make sure that's working as intended.
	repo.Git(t, "checkout", "-b", "stack-1")
	repo.CommitFile(t, "my-file", "1a\n", gittest.WithMessage("Commit 1a"))
	RequireAv(t, "stack", "branch", "stack-2")
	repo.CommitFile(t, "my-file", "1a\n2a\n", gittest.WithMessage("Commit 2a"))
	RequireAv(t, "stack", "branch", "stack-3")
	repo.CommitFile(
		t,
		"different-file",
		"1a\n2a\n3a\n",
		gittest.WithMessage("Commit 3a"),
	)
	repo.Git(t, "checkout", "stack-1")
	RequireAv(t, "stack", "branch", "stack-4")
	repo.CommitFile(t, "another-file", "1a\n4a\n", gittest.WithMessage("Commit 4a"))
	repo.Git(t, "checkout", "stack-3")

	// Everything up to date now, so this should be a no-op.
	RequireAv(t, "stack", "restack")

	// We're going to add a commit to the first branch in the stack.
	// Our stack looks like:
	//      stack-1: main -> 1a -> 1b
	//      stack-2:           \ -> 2a
	//      stack-3:           |    \ -> 3a
	//      stack-4:           \ -> 4a

	// (note that stack-2 has diverged with stack-1)
	// Ultimately, after the sync (and resolving conflicts), our stack should look like:
	//      stack-1: main -> 1a -> 1b
	//      stack-2:                 \ -> 2a'
	//      stack-3:                 |     \ -> 3a'
	//      stack-4:                 \ -> 4a'
	// It's very important here to make sure to handle the sync of stack-3 correctly.
	// After syncing stack-2 onto stack-1 (before syncinc stack-3), our commit
	// graph looks like:
	//      stack-1: main -> 1a -> 1b
	//      stack-2:                 \ -> 2a'
	//      stack-3:          \ -> 2a -> 3a
	//      stack-4:          \ -> 4a

	// (remember that we haven't yet modified stack-3, so 3a still has parent 2a,
	// but 2a is actually not even associated with stack-2 anymore since we had
	// to rebase sync it on top of 1b, creating new commit 2a').
	// If we do this naively (trying to rebase stack-3 on top of 2a'), Git will
	// find every commit that is reachable from 3a but not 2a' (in this case,
	// that's 2a and 3a) and replay those commits on top of 2a'. The net result
	// is that we've duplicated 2a (and it's likely to have conflicts at that).
	// A naive `git rebase stack-2` won't work. Instead we need to make sure to
	// do `git rebase --onto 2a' 2a` instead (which says look at every
	// commit since 2a and play it on top of 2a').
	// This also applies to any situation where the user has modified a commit
	// that was stacked-upon (e.g., with `git commit --amend`).
	repo.WithCheckoutBranch(t, "refs/heads/stack-1", func() {
		repo.CommitFile(t, "my-file", "1a\n1b\n", gittest.WithMessage("Commit 1b"))
	})

	// Since both commits updated my-file in ways that conflict, we should get
	// a merge/rebase conflict here.
	syncConflict := Av(t, "stack", "restack")
	require.NotEqual(
		t, 0, syncConflict.ExitCode,
		"stack restack should return non-zero exit code if conflicts",
	)
	require.Contains(
		t, syncConflict.Stdout,
		"error: could not apply", "stack restack should include error message on rebase",
	)
	require.Contains(
		t, syncConflict.Stdout, "av stack restack --continue",
		"stack restack should print a message with instructions to continue",
	)
	syncContinueWithoutResolving := Av(t, "stack", "restack", "--continue")
	require.NotEqual(
		t,
		0,
		syncContinueWithoutResolving.ExitCode,
		"stack restack --continue should return non-zero exit code if conflicts have not been resolved",
	)
	// resolve the conflict
	err := os.WriteFile(filepath.Join(repo.RepoDir, "my-file"), []byte("1a\n1b\n2a\n"), 0644)
	require.NoError(t, err)
	repo.Git(t, "add", "my-file")
	require.NoError(t, err, "failed to stage file")
	// stack restack --continue should return zero exit code after resolving conflicts
	RequireAv(t, "stack", "restack", "--continue")

	// Make sure we've handled the rebase of stack-3 correctly (see the long
	// comment above).
	commits := repo.GetCommits(t, plumbing.NewBranchReferenceName("stack-3"), plumbing.NewBranchReferenceName("stack-2"))
	require.Len(t, commits, 1)

	mergeBases := repo.MergeBase(t, plumbing.NewBranchReferenceName("stack-1"), plumbing.NewBranchReferenceName("stack-2"))
	stack1Head := repo.GetCommitAtRef(t, plumbing.NewBranchReferenceName("stack-1"))
	require.Equal(t, mergeBases[0], stack1Head, "stack-2 should be up-to-date with stack-1")

	// Further sync attemps should yield no-ops
	syncNoop := RequireAv(t, "stack", "restack")
	require.Contains(t, syncNoop.Stdout, "Restack done")

	// Make sure we've not introduced any extra commits
	// We should have 4 (corresponding to 1a, 1b, 2a, and 3a).
	commits = repo.GetCommits(t, plumbing.NewBranchReferenceName("stack-3"), plumbing.NewBranchReferenceName("main"))
	require.NoError(t, err)
	require.Len(t, commits, 4)

	stack1Commit := repo.GetCommitAtRef(t, plumbing.NewBranchReferenceName("stack-1"))
	stack2Commit := repo.GetCommitAtRef(t, plumbing.NewBranchReferenceName("stack-2"))

	require.Equal(t, meta.BranchState{
		Name:  "main",
		Trunk: true,
	}, GetStoredParentBranchState(t, repo, "stack-1"))
	require.Equal(t, meta.BranchState{
		Name: "stack-1",
		Head: stack1Commit.String(),
	}, GetStoredParentBranchState(t, repo, "stack-2"))
	require.Equal(t, meta.BranchState{
		Name: "stack-2",
		Head: stack2Commit.String(),
	}, GetStoredParentBranchState(t, repo, "stack-3"))
	require.Equal(t, meta.BranchState{
		Name: "stack-1",
		Head: stack1Commit.String(),
	}, GetStoredParentBranchState(t, repo, "stack-4"))
}

func TestStackRestackAbort(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	// Create a two stack...
	repo.Git(t, "checkout", "-b", "stack-1")
	repo.CommitFile(t, "my-file", "1a\n", gittest.WithMessage("Commit 1a"))
	RequireAv(t, "stack", "branch", "stack-2")
	repo.CommitFile(t, "my-file", "1a\n2a\n", gittest.WithMessage("Commit 2a"))

	// Save the original parent HEAD for stack-2, which is the stack-1's commit.
	origStack1Commit := repo.GetCommitAtRef(t, plumbing.NewBranchReferenceName("stack-1"))

	// ... and introduce a commit onto stack-1 that will conflict with stack-2...
	repo.CheckoutBranch(t, "refs/heads/stack-1")
	repo.CommitFile(t, "my-file", "1a\n1b\n", gittest.WithMessage("Commit 1b"))

	// ... and make sure we get a conflict on sync...
	syncConflict := Av(t, "stack", "restack")
	require.NotEqual(
		t,
		0,
		syncConflict.ExitCode,
		"stack restack should return non-zero exit code if conflicts",
	)
	require.FileExists(
		t,
		filepath.Join(repo.GitDir, "REBASE_HEAD"),
		"REBASE_HEAD should be created for conflict",
	)

	// ... and then abort the sync...
	RequireAv(t, "stack", "restack", "--abort")
	require.NoFileExists(
		t,
		filepath.Join(repo.GitDir, "REBASE_HEAD"),
		"REBASE_HEAD should be removed after abort",
	)

	// ... and make sure that we return to stack-1 (where we started).
	// (this also makes sure that we've actually aborted the rebase and are not
	// in a detached HEAD state).
	require.Equal(
		t,
		plumbing.ReferenceName("refs/heads/stack-1"),
		repo.CurrentBranch(t),
		"current branch should be reset to starting branch (stack-1) after abort",
	)

	// Because we aborted the sync, the stack-2 parent HEAD must stay at the original stack-1
	// HEAD.
	require.Equal(t, meta.BranchState{
		Name: "stack-1",
		Head: origStack1Commit.String(),
	}, GetStoredParentBranchState(t, repo, "stack-2"))
}

func TestStackRestackWithLotsOfConflicts(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	// Create a three stack...
	repo.Git(t, "checkout", "-b", "stack-1")
	repo.CommitFile(t, "my-file", "1a\n", gittest.WithMessage("Commit 1a"))
	RequireAv(t, "stack", "branch", "stack-2")
	repo.CommitFile(t, "my-file", "1a\n2a\n", gittest.WithMessage("Commit 2a"))
	RequireAv(t, "stack", "branch", "stack-3")
	repo.CommitFile(t, "my-file", "1a\n2a\n3a\n", gittest.WithMessage("Commit 3a"))

	// Go back to the first branch (to make sure that the sync constructs the
	// list of branches correctly).
	repo.CheckoutBranch(t, "refs/heads/stack-1")

	// Add new conflicting commits to each branch
	repo.WithCheckoutBranch(t, "refs/heads/stack-1", func() {
		repo.CommitFile(t, "my-file", "1a\n1b\n", gittest.WithMessage("Commit 1b"))
	})
	repo.WithCheckoutBranch(t, "refs/heads/stack-2", func() {
		repo.CommitFile(
			t,
			"my-file",
			"1a\n2a\n2b\n",
			gittest.WithMessage("Commit 2b"),
		)
	})
	repo.WithCheckoutBranch(t, "refs/heads/stack-3", func() {
		repo.CommitFile(
			t,
			"my-file",
			"1a\n2a\n3a\n3b\n",
			gittest.WithMessage("Commit 3b"),
		)
	})

	sync := Av(t, "stack", "restack")
	require.NotEqual(
		t,
		0,
		sync.ExitCode,
		"stack restack should return non-zero exit code if conflicts",
	)
	require.Regexp(t, regexp.MustCompile("could not apply .+ Commit 2a"), sync.Stdout)
	require.NoError(t, os.WriteFile("my-file", []byte("1a\n1b\n2a\n"), 0644))
	repo.Git(t, "add", "my-file")

	// Commit 2b should be able to be applied normally, then we should have a
	// conflict with 3a
	sync = Av(t, "stack", "restack", "--continue")
	require.NotEqual(
		t,
		0,
		sync.ExitCode,
		"stack restack should return non-zero exit code if conflicts",
	)
	require.Regexp(t, regexp.MustCompile("could not apply .+ Commit 3a"), sync.Stdout)
	require.NoError(t, os.WriteFile("my-file", []byte("1a\n1b\n2a\n2b\n3a\n"), 0644))
	repo.Git(t, "add", "my-file")

	// And finally, 3b should be able to be applied without conflict and our stack
	// sync should be over.
	RequireAv(t, "stack", "restack", "--continue")
}

func TestStackRestackAfterAmendingCommit(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.RepoDir)

	// Create a three stack...
	repo.Git(t, "checkout", "-b", "stack-1")
	repo.CommitFile(t, "my-file", "1a\n", gittest.WithMessage("Commit 1a"))
	repo.CommitFile(t, "my-file", "1a\n1b\n", gittest.WithMessage("Commit 1b"))
	RequireAv(t, "stack", "branch", "stack-2")
	repo.CommitFile(t, "my-file", "1a\n1b\n2a\n", gittest.WithMessage("Commit 2a"))
	repo.CommitFile(
		t,
		"my-file",
		"1a\n1b\n2a\n2b\n",
		gittest.WithMessage("Commit 2b"),
	)
	RequireAv(t, "stack", "branch", "stack-3")
	repo.CommitFile(
		t,
		"my-file",
		"1a\n1b\n2a\n2b\n3a\n",
		gittest.WithMessage("Commit 3a"),
	)
	repo.CommitFile(
		t,
		"my-file",
		"1a\n1b\n2a\n2b\n3a\n3b\n",
		gittest.WithMessage("Commit 3b"),
	)

	// Now we amend commit 1b and make sure the sync after succeeds
	repo.CheckoutBranch(t, "refs/heads/stack-1")
	repo.CommitFile(t, "my-file", "1a\n1c\n1b\n", gittest.WithAmend())
	RequireAv(t, "stack", "restack")

	repo.CheckoutBranch(t, "refs/heads/stack-3")
	contents, err := os.ReadFile("my-file")
	require.NoError(t, err)
	require.Equal(t, "1a\n1c\n1b\n2a\n2b\n3a\n3b\n", string(contents))

	// Now we amend commit 2a and make sure the sync succeeds
	repo.CheckoutBranch(t, "refs/heads/stack-2")
}
