package e2e_tests

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta"
	"github.com/stretchr/testify/require"
)

func TestHelp(t *testing.T) {
	res := Av(t, "--help")
	require.Equal(t, 0, res.ExitCode)
}

func TestStackSync(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.Dir())

	// To start, we create a simple three-stack where each stack has a single commit.
	// Our stack looks like:
	//     stack-1: main -> 1a
	//     stack-2:           \ -> 2a
	//     stack-3:                \ -> 3a
	// Note: we create the first branch with a "vanilla" git checkout just to
	// make sure that's working as intended.
	require.Equal(t, 0, Cmd(t, "git", "checkout", "-b", "stack-1").ExitCode)
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n"), gittest.WithMessage("Commit 1a"))
	require.Equal(t, 0, Av(t, "stack", "branch", "stack-2").ExitCode)
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n2a\n"), gittest.WithMessage("Commit 2a"))
	require.Equal(t, 0, Av(t, "stack", "branch", "stack-3").ExitCode)
	gittest.CommitFile(
		t,
		repo,
		"different-file",
		[]byte("1a\n2a\n3a\n"),
		gittest.WithMessage("Commit 3a"),
	)

	// Everything up to date now, so this should be a no-op.
	require.Equal(t, 0, Av(t, "stack", "sync", "--no-fetch", "--no-push").ExitCode)

	// We're going to add a commit to the first branch in the stack.
	// Our stack looks like:
	//      stack-1: main -> 1a -> 1b
	//      stack-2:           \ -> 2a
	//      stack-3:                \ -> 3a

	// (note that stack-2 has diverged with stack-1)
	// Ultimately, after the sync (and resolving conflicts), our stack should look like:
	//      stack-1: main -> 1a -> 1b
	//      stack-2:                 \ -> 2a'
	//      stack-3:                       \ -> 3a'
	// It's very important here to make sure to handle the sync of stack-3 correctly.
	// After syncing stack-2 onto stack-1 (before syncinc stack-3), our commit
	// graph looks like:
	//      stack-1: main -> 1a -> 1b
	//      stack-2:                 \ -> 2a'
	//      stack-3:          \ -> 2a -> 3a

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
	gittest.WithCheckoutBranch(t, repo, "stack-1", func() {
		gittest.CommitFile(t, repo, "my-file", []byte("1a\n1b\n"), gittest.WithMessage("Commit 1b"))
	})

	// Since both commits updated my-file in ways that conflict, we should get
	// a merge/rebase conflict here.
	syncConflict := Av(t, "stack", "sync", "--no-fetch", "--no-push")
	require.NotEqual(
		t, 0, syncConflict.ExitCode,
		"stack sync should return non-zero exit code if conflicts",
	)
	require.Contains(
		t, syncConflict.Stderr,
		"error: could not apply", "stack sync should include error message on rebase",
	)
	require.Contains(
		t, syncConflict.Stderr, "av stack sync --continue",
		"stack sync should print a message with instructions to continue",
	)
	syncContinueWithoutResolving := Av(t, "stack", "sync", "--continue")
	require.NotEqual(
		t,
		0,
		syncContinueWithoutResolving.ExitCode,
		"stack sync --continue should return non-zero exit code if conflicts have not been resolved",
	)
	// resolve the conflict
	err := os.WriteFile(filepath.Join(repo.Dir(), "my-file"), []byte("1a\n1b\n2a\n"), 0644)
	require.NoError(t, err)
	_, err = repo.Git("add", "my-file")
	require.NoError(t, err, "failed to stage file")
	syncContinue := Av(t, "stack", "sync", "--continue")
	require.Equal(
		t,
		0,
		syncContinue.ExitCode,
		"stack sync --continue should return zero exit code after resolving conflicts",
	)

	// Make sure we've handled the rebase of stack-3 correctly (see the long
	// comment above).
	revs, err := repo.RevList(git.RevListOpts{
		Specifiers: []string{"stack-2..stack-3"},
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(revs))

	mergeBase, err := repo.MergeBase(&git.MergeBase{Revs: []string{"stack-1", "stack-2"}})
	require.NoError(t, err)
	stack1Head, err := repo.RevParse(&git.RevParse{Rev: "stack-1"})
	require.NoError(t, err)
	require.Equal(t, mergeBase, stack1Head, "stack-2 should be up-to-date with stack-1")

	// Further sync attemps should yield no-ops
	syncNoop := Av(t, "stack", "sync", "--no-fetch", "--no-push")
	require.Equal(t, 0, syncNoop.ExitCode)
	require.Contains(t, syncNoop.Stderr, "already up-to-date")

	// Make sure we've not introduced any extra commits
	// We should have 4 (corresponding to 1a, 1b, 2a, and 3a).
	revs, err = repo.RevList(git.RevListOpts{
		Specifiers: []string{"main..stack-3"},
	})
	require.NoError(t, err)
	require.Equal(t, 4, len(revs))

	stack1Commit, err := repo.RevParse(&git.RevParse{Rev: "stack-1"})
	require.NoError(t, err)

	stack2Commit, err := repo.RevParse(&git.RevParse{Rev: "stack-2"})
	require.NoError(t, err)

	require.Equal(t, meta.BranchState{
		Name:  "main",
		Trunk: true,
	}, GetStoredParentBranchState(t, repo, "stack-1"))
	require.Equal(t, meta.BranchState{
		Name: "stack-1",
		Head: stack1Commit,
	}, GetStoredParentBranchState(t, repo, "stack-2"))
	require.Equal(t, meta.BranchState{
		Name: "stack-2",
		Head: stack2Commit,
	}, GetStoredParentBranchState(t, repo, "stack-3"))
}

func TestStackSyncAbort(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.Dir())

	// Create a two stack...
	RequireCmd(t, "git", "checkout", "-b", "stack-1")
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n"), gittest.WithMessage("Commit 1a"))
	RequireAv(t, "stack", "branch", "stack-2")
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n2a\n"), gittest.WithMessage("Commit 2a"))

	// Save the original parent HEAD for stack-2, which is the stack-1's commit.
	origStack1Commit, err := repo.RevParse(&git.RevParse{Rev: "stack-1"})
	require.NoError(t, err)

	// ... and introduce a commit onto stack-1 that will conflict with stack-2...
	gittest.CheckoutBranch(t, repo, "stack-1")
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n1b\n"), gittest.WithMessage("Commit 1b"))

	// ... and make sure we get a conflict on sync...
	syncConflict := Av(t, "stack", "sync", "--no-fetch", "--no-push")
	require.NotEqual(
		t,
		0,
		syncConflict.ExitCode,
		"stack sync should return non-zero exit code if conflicts",
	)
	require.FileExists(
		t,
		path.Join(repo.GitDir(), "REBASE_HEAD"),
		"REBASE_HEAD should be created for conflict",
	)

	// ... and then abort the sync...
	RequireAv(t, "stack", "sync", "--abort")
	require.NoFileExists(
		t,
		path.Join(repo.GitDir(), "REBASE_HEAD"),
		"REBASE_HEAD should be removed after abort",
	)

	// ... and make sure that we return to stack-1 (where we started).
	// (this also makes sure that we've actually aborted the rebase and are not
	// in a detached HEAD state).
	currentBranch, err := repo.RevParse(&git.RevParse{Rev: "HEAD", SymbolicFullName: true})
	require.NoError(t, err, "failed to get current branch")
	require.Equal(
		t,
		"refs/heads/stack-1",
		currentBranch,
		"current branch should be reset to starting branch (stack-1) after abort",
	)

	// Because we aborted the sync, the stack-2 parent HEAD must stay at the original stack-1
	// HEAD.
	require.Equal(t, meta.BranchState{
		Name: "stack-1",
		Head: origStack1Commit,
	}, GetStoredParentBranchState(t, repo, "stack-2"))
}

func TestStackSyncWithLotsOfConflicts(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	Chdir(t, repo.Dir())

	// Create a three stack...
	RequireCmd(t, "git", "checkout", "-b", "stack-1")
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n"), gittest.WithMessage("Commit 1a"))
	RequireAv(t, "stack", "branch", "stack-2")
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n2a\n"), gittest.WithMessage("Commit 2a"))
	RequireAv(t, "stack", "branch", "stack-3")
	gittest.CommitFile(t, repo, "my-file", []byte("1a\n2a\n3a\n"), gittest.WithMessage("Commit 3a"))

	// Go back to the first branch (to make sure that the sync constructs the
	// list of branches correctly).
	gittest.CheckoutBranch(t, repo, "stack-1")

	// Add new conflicting commits to each branch
	gittest.WithCheckoutBranch(t, repo, "stack-1", func() {
		gittest.CommitFile(t, repo, "my-file", []byte("1a\n1b\n"), gittest.WithMessage("Commit 1b"))
	})
	gittest.WithCheckoutBranch(t, repo, "stack-2", func() {
		gittest.CommitFile(
			t,
			repo,
			"my-file",
			[]byte("1a\n2a\n2b\n"),
			gittest.WithMessage("Commit 2b"),
		)
	})
	gittest.WithCheckoutBranch(t, repo, "stack-3", func() {
		gittest.CommitFile(
			t,
			repo,
			"my-file",
			[]byte("1a\n2a\n3a\n3b\n"),
			gittest.WithMessage("Commit 3b"),
		)
	})

	sync := Av(t, "stack", "sync", "--no-fetch", "--no-push")
	require.NotEqual(
		t,
		0,
		sync.ExitCode,
		"stack sync should return non-zero exit code if conflicts",
	)
	require.Regexp(t, regexp.MustCompile("could not apply .+ Commit 2a"), sync.Stderr)
	require.NoError(t, os.WriteFile("my-file", []byte("1a\n1b\n2a\n"), 0644))
	RequireCmd(t, "git", "add", "my-file")

	// Commit 2b should be able to be applied normally, then we should have a
	// conflict with 3a
	sync = Av(t, "stack", "sync", "--continue")
	require.NotEqual(
		t,
		0,
		sync.ExitCode,
		"stack sync should return non-zero exit code if conflicts",
	)
	require.Regexp(t, regexp.MustCompile("could not apply .+ Commit 3a"), sync.Stderr)
	require.NoError(t, os.WriteFile("my-file", []byte("1a\n1b\n2a\n2b\n3a\n"), 0644))
	RequireCmd(t, "git", "add", "my-file")

	// And finally, 3b should be able to be applied without conflict and our stack
	// sync should be over.
	sync = Av(t, "stack", "sync", "--continue")
	require.Equal(t, 0, sync.ExitCode, "stack sync should return zero exit code if no conflicts")
}
