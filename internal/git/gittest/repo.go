package gittest

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	avgit "github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/kr/text"
	"github.com/stretchr/testify/require"
)

// NewTempRepo initializes a new git repository with reasonable defaults.
func NewTempRepo(t *testing.T) *GitTestRepo {
	t.Helper()
	return NewTempRepoWithGitHubServer(t, "http://github.invalid")
}

func NewTempRepoWithGitHubServer(t *testing.T, serverURL string) *GitTestRepo {
	t.Helper()
	var dir string
	var remoteDir string
	if os.Getenv("AV_TEST_PRESERVE_TEMP_REPO") != "" {
		var err error
		dir, err = os.MkdirTemp("", "repo") //nolint:usetesting
		require.NoError(t, err)
		t.Logf("Created git test repo: %s", dir)

		remoteDir, err = os.MkdirTemp("", "remote-repo") //nolint:usetesting
		require.NoError(t, err)
		t.Logf("Created git remote test repo: %s", remoteDir)
	} else {
		dir = filepath.Join(t.TempDir(), "local")
		require.NoError(t, os.MkdirAll(dir, 0o755))

		remoteDir = filepath.Join(t.TempDir(), "remote")
		require.NoError(t, os.MkdirAll(remoteDir, 0o755))
	}
	init := exec.CommandContext(t.Context(), "git", "init", "--initial-branch=main")
	init.Dir = dir

	err := init.Run()
	require.NoError(t, err, "failed to initialize git repository")

	remoteInit := exec.CommandContext(t.Context(), "git", "init", "--bare")
	remoteInit.Dir = remoteDir

	err = remoteInit.Run()
	require.NoError(t, err, "failed to initialize remote git repository")

	ggRepo, err := git.PlainOpen(dir)
	require.NoError(t, err, "failed to open git repository")

	repo := &GitTestRepo{dir, filepath.Join(dir, ".git"), ggRepo}
	require.NoError(t, err, "failed to open repo")

	settings := map[string]string{
		"user.name":  "av-test",
		"user.email": "av-test@nonexistent",
	}
	for k, v := range settings {
		repo.Git(t, "config", k, v)
	}

	repo.Git(t, "remote", "add", "origin", remoteDir, "--master=main")

	err = os.WriteFile(dir+"/README.md", []byte("# Hello World"), 0o644)
	require.NoError(t, err, "failed to write README.md")

	repo.Git(t, "add", "README.md")
	repo.Git(t, "commit", "-m", "Initial commit")
	repo.Git(t, "push", "origin", "main")

	// Write metadata because some commands expect it to be there.
	// This repository obviously doesn't exist so tests still need to be careful
	// not to invoke operations that would communicate with GitHub (e.g.,
	// by using the `--no-fetch` and `--no-push` flags).
	db, _, err := jsonfiledb.OpenPath(filepath.Join(repo.GitDir, "av", "av.db"))
	if err != nil {
		require.NoError(t, err, "failed to open database")
	}
	tx := db.WriteTx()
	tx.SetRepository(meta.Repository{
		ID:    "R_nonexistent_",
		Owner: "aviator-co",
		Name:  "nonexistent",
	})
	require.NoError(t, tx.Commit(), "failed to write repository metadata")

	err = os.WriteFile(
		filepath.Join(repo.GitDir, "av", "config.yml"),
		fmt.Appendf(nil, `
github:
    token: "dummy_valid_token"
    baseUrl: %q
`, serverURL),
		0o644,
	)
	require.NoError(t, err, "failed to write .git/av/config.yml")

	return repo
}

type GitTestRepo struct {
	RepoDir string
	GitDir  string
	GoGit   *git.Repository
}

func (r *GitTestRepo) AsAvGitRepo() *avgit.Repo {
	repo, _ := avgit.OpenRepo(r.RepoDir, r.GitDir)
	return repo
}

func (r *GitTestRepo) Git(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Dir = r.RepoDir
	err := cmd.Run()
	var exitError *exec.ExitError
	if err != nil && !errors.As(err, &exitError) {
		t.Fatal(err)
	}
	t.Logf("Running git\n"+
		"args: %v\n"+
		"exit code: %v\n"+
		"stdout:\n"+
		"%s"+
		"stderr:\n"+
		"%s",
		args,
		cmd.ProcessState.ExitCode(),
		text.Indent(stdout.String(), "  "),
		text.Indent(stderr.String(), "  "),
	)
	return stdout.String()
}

func (r *GitTestRepo) OpenDB(t *testing.T) *jsonfiledb.DB {
	t.Helper()
	db, _, err := jsonfiledb.OpenPath(filepath.Join(r.GitDir, "av", "av.db"))
	require.NoError(t, err, "failed to open database")
	return db
}

func (r *GitTestRepo) AddFile(t *testing.T, fp string) {
	t.Helper()
	r.Git(t, "add", fp)
}

func (r *GitTestRepo) CreateFile(t *testing.T, filename string, body string) string {
	t.Helper()
	fp := filepath.Join(r.RepoDir, filename)
	err := os.WriteFile(fp, []byte(body), 0o644)
	require.NoError(t, err, "failed to write file: %s", filename)
	return fp
}

type commitFileOpts struct {
	msg   string
	amend bool
}

type CommitFileOpt func(*commitFileOpts)

func WithMessage(msg string) CommitFileOpt {
	return func(opts *commitFileOpts) {
		opts.msg = msg
	}
}

func WithAmend() CommitFileOpt {
	return func(opts *commitFileOpts) {
		opts.amend = true
	}
}

func (r *GitTestRepo) CommitFile(
	t *testing.T,
	filename string,
	body string,
	cfOpts ...CommitFileOpt,
) plumbing.Hash {
	t.Helper()
	opts := commitFileOpts{
		msg: fmt.Sprintf("Write %s", filename),
	}
	for _, o := range cfOpts {
		o(&opts)
	}

	filepath := r.CreateFile(t, filename, body)
	r.AddFile(t, filepath)

	args := []string{"commit", "-m", opts.msg}
	if opts.amend {
		args = append(args, "--amend")
	}
	r.Git(t, args...)
	headRef, err := r.GoGit.Head()
	require.NoError(t, err, "failed to get HEAD")
	return headRef.Hash()
}

func (r *GitTestRepo) IsWorkdirClean(t *testing.T) bool {
	t.Helper()
	return r.Git(t, "status", "--porcelain") == ""
}

func (r *GitTestRepo) CurrentBranch(t *testing.T) plumbing.ReferenceName {
	t.Helper()
	head, err := r.GoGit.Head()
	require.NoError(t, err, "failed to get HEAD")
	return head.Name()
}

func (r *GitTestRepo) GetCommitAtRef(t *testing.T, name plumbing.ReferenceName) plumbing.Hash {
	t.Helper()
	ref, err := r.GoGit.Reference(name, true)
	require.NoError(t, err, "failed to get a ref at %q", name)
	return ref.Hash()
}

func (r *GitTestRepo) CreateRef(t *testing.T, ref plumbing.ReferenceName) {
	t.Helper()
	head, err := r.GoGit.Head()
	require.NoError(t, err, "failed to get HEAD")

	err = r.GoGit.Storer.SetReference(plumbing.NewHashReference(ref, head.Hash()))
	require.NoError(t, err, "failed to create branch %q", ref)
}

// CheckoutBranch checks out the specified branch and returns the original branch.
func (r *GitTestRepo) CheckoutBranch(
	t *testing.T,
	branch plumbing.ReferenceName,
) plumbing.ReferenceName {
	t.Helper()
	original := r.CurrentBranch(t)
	wt, err := r.GoGit.Worktree()
	require.NoError(t, err, "failed to get worktree")
	err = wt.Checkout(&git.CheckoutOptions{Branch: branch})
	require.NoError(t, err, "failed to checkout branch")
	return original
}

func (r *GitTestRepo) CheckoutCommit(t *testing.T, hash plumbing.Hash) {
	t.Helper()
	wt, err := r.GoGit.Worktree()
	require.NoError(t, err, "failed to get worktree")
	err = wt.Checkout(&git.CheckoutOptions{Hash: hash})
	require.NoError(t, err, "failed to checkout branch")
}

// WithCheckoutBranch runs the given function after checking out the specified branch.
// It returns to the original branch after the function returns.
func (r *GitTestRepo) WithCheckoutBranch(t *testing.T, branch plumbing.ReferenceName, f func()) {
	t.Helper()
	original := r.CheckoutBranch(t, branch)
	defer r.CheckoutBranch(t, original)
	f()
}

func (r *GitTestRepo) GetCommits(
	t *testing.T,
	includedFromRef, excludedFromRef plumbing.ReferenceName,
) []plumbing.Hash {
	t.Helper()
	from := r.GetCommitAtRef(t, includedFromRef)
	excluded := r.GetCommitAtRef(t, excludedFromRef)

	commit, err := r.GoGit.CommitObject(from)
	require.NoError(t, err, "failed to get commit at %q", from)

	commits := []plumbing.Hash{}
	commitIter := object.NewCommitPreorderIter(commit, nil, []plumbing.Hash{excluded})
	err = commitIter.ForEach(func(c *object.Commit) error {
		commits = append(commits, c.Hash)
		return nil
	})
	require.NoError(t, err, "failed to iterate commits")
	return commits
}

func (r *GitTestRepo) MergeBase(t *testing.T, ref1, ref2 plumbing.ReferenceName) []plumbing.Hash {
	t.Helper()
	c1, err := r.GoGit.CommitObject(r.GetCommitAtRef(t, ref1))
	require.NoError(t, err, "failed to get commit at %q", ref1)
	c2, err := r.GoGit.CommitObject(r.GetCommitAtRef(t, ref2))
	require.NoError(t, err, "failed to get commit at %q", ref2)

	bases, err := c1.MergeBase(c2)
	require.NoError(t, err, "failed to get merge bases")
	var ret []plumbing.Hash
	for _, c := range bases {
		ret = append(ret, c.Hash)
	}
	return ret
}
