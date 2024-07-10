package git

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/config"
	"github.com/go-git/go-git/v5"
	"github.com/sirupsen/logrus"
	giturls "github.com/whilp/git-urls"
)

var ErrRemoteNotFound = errors.Sentinel("this repository doesn't have a remote origin")

const DEFAULT_REMOTE_NAME = "origin"

type Repo struct {
	repoDir string
	gitDir  string
	gitRepo *git.Repository
	log     logrus.FieldLogger
}

func OpenRepo(repoDir string, gitDir string) (*Repo, error) {
	repo, err := git.PlainOpenWithOptions(repoDir, &git.PlainOpenOptions{
		DetectDotGit:          true,
		EnableDotGitCommonDir: true,
	})
	if err != nil {
		return nil, errors.Errorf("failed to open git repo: %v", err)
	}
	r := &Repo{
		repoDir,
		gitDir,
		repo,
		logrus.WithFields(logrus.Fields{"repo": filepath.Base(repoDir)}),
	}
	return r, nil
}

func (r *Repo) Dir() string {
	return r.repoDir
}

func (r *Repo) GitDir() string {
	return r.gitDir
}

func (r *Repo) AvDir() string {
	return filepath.Join(r.GitDir(), "av")
}

func (r *Repo) GoGitRepo() *git.Repository {
	return r.gitRepo
}

func (r *Repo) AvTmpDir() string {
	dir := filepath.Join(r.AvDir(), "tmp")
	// Try to create the directory, but swallow the error since it will
	// ultimately be surfaced when trying to create a file in the directory.
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func (r *Repo) DefaultBranch() (string, error) {
	ref, err := r.Git("symbolic-ref", "refs/remotes/origin/HEAD")
	if err != nil {
		logrus.WithError(err).Debug("failed to determine remote HEAD")
		// this communicates with the remote, so we probably don't want to run
		// it by default, but we helpfully suggest it to the user. :shrug:
		logrus.Warn(
			"Failed to determine repository default branch. " +
				"Ensure you have a remote named origin and try running `git remote set-head --auto origin` to fix this.",
		)
		return "", errors.New("failed to determine remote HEAD")
	}
	return strings.TrimPrefix(ref, "refs/remotes/origin/"), nil
}

func (r *Repo) IsTrunkBranch(name string) (bool, error) {
	branches, err := r.TrunkBranches()
	if err != nil {
		return false, err
	}
	for _, branch := range branches {
		if name == branch {
			return true, nil
		}
	}
	return false, nil
}

func (r *Repo) TrunkBranches() ([]string, error) {
	defaultBranch, err := r.DefaultBranch()
	if err != nil {
		return nil, err
	}

	branches := []string{defaultBranch}
	branches = append(branches, config.Av.AdditionalTrunkBranches...)

	return branches, nil
}

func (r *Repo) GetRemoteName() string {
	if config.Av.Remote != "" {
		return config.Av.Remote
	}

	return DEFAULT_REMOTE_NAME
}

func (r *Repo) Git(args ...string) (string, error) {
	startTime := time.Now()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.repoDir
	out, err := cmd.Output()
	log := r.log.WithField("duration", time.Since(startTime))
	if err != nil {
		stderr := "<no output>"
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			stderr = string(exitError.Stderr)
		}
		log.Debugf("git %s failed: %s: %s", args, err, stderr)
		return strings.TrimSpace(string(out)), errors.Wrapf(err, "git %s", args[0])
	}

	// trim trailing newline
	log.Debugf("git %s", args)
	return strings.TrimSpace(string(out)), nil
}

type RunOpts struct {
	Args []string
	Env  []string
	// If true, return a non-nil error if the command exited with a non-zero
	// exit code.
	ExitError bool
	// If true, the standard I/Os are connected to the console, allowing the git command to
	// interact with the user. Stdout and Stderr will be empty.
	Interactive bool
	// The standard input to the command (if any). Mutually exclusive with Interactive.
	Stdin io.Reader
}

type Output struct {
	ExitCode  int
	ExitError *exec.ExitError
	Stdout    []byte
	Stderr    []byte
}

func (o Output) Lines() []string {
	s := strings.TrimSpace(string(o.Stdout))
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func (r *Repo) Run(opts *RunOpts) (*Output, error) {
	cmd := exec.Command("git", opts.Args...)
	cmd.Dir = r.repoDir
	r.log.Debugf("git %s", opts.Args)
	var stdout, stderr bytes.Buffer
	if opts.Interactive {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}
	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	}
	cmd.Env = append(os.Environ(), opts.Env...)
	err := cmd.Run()
	var exitError *exec.ExitError
	if err != nil && !errors.As(err, &exitError) {
		return nil, errors.Wrapf(err, "git %s", opts.Args)
	}
	if err != nil && opts.ExitError && exitError.ExitCode() != 0 {
		// ExitError.Stderr is only populated if the command was started without
		// a Stderr pipe, which is not the case here. Just populate it ourselves
		// to make it easier for callers to access.
		exitError.Stderr = stderr.Bytes()
		return nil, errors.WrapIff(err, "git %s (%s)", opts.Args, stderr.String())
	}
	return &Output{
		ExitCode:  cmd.ProcessState.ExitCode(),
		ExitError: exitError,
		Stdout:    stdout.Bytes(),
		Stderr:    stderr.Bytes(),
	}, nil
}

// DetachedHead returns true if the repository is in the detached HEAD.
func (r *Repo) DetachedHead() (bool, error) {
	ret, err := r.Run(&RunOpts{
		Args: []string{
			"symbolic-ref", "--quiet", "HEAD",
		},
	})
	if err != nil {
		return false, err
	}
	return ret.ExitCode == 1, nil
}

// CurrentBranchName returns the name of the current branch.
// The name is return in "short" format -- i.e., without the "refs/heads/" prefix.
// IMPORTANT: This function will return an error if the repository is currently
// in a detached-head state (e.g., during a rebase conflict).
func (r *Repo) CurrentBranchName() (string, error) {
	branch, err := r.Git("symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", errors.Wrap(
			err,
			"failed to determine current branch (are you in detached HEAD or is a rebase in progress?)",
		)
	}
	return branch, nil
}

// CheckCleanWorkdir returns if the workdir is clean.
func (r *Repo) CheckCleanWorkdir() (bool, error) {
	out, err := r.Git("status", "--porcelain")
	if err != nil {
		return false, errors.Errorf("failed to check if the working directory is clean: %v", err)
	}
	return out == "", nil
}

// HasChangesToBeCommitted returns if there's a staged changes to be committed.
func (r *Repo) HasChangesToBeCommitted() (bool, error) {
	out, err := r.Run(&RunOpts{
		Args: []string{"diff-index", "--quiet", "--cached", "HEAD"},
	})
	if err != nil {
		return false, errors.Errorf("failed to check if there are changes to be committed: %v", err)
	}
	if out.ExitCode != 0 && out.ExitCode != 1 {
		return false, errors.Errorf(
			"failed to check if there are changes to be committed: exit code %d",
			out.ExitCode,
		)
	}
	return out.ExitCode == 1, nil
}

func (r *Repo) DoesBranchExist(branch string) (bool, error) {
	return r.DoesRefExist(fmt.Sprintf("refs/heads/%s", branch))
}

func (r *Repo) DoesRemoteBranchExist(branch string) (bool, error) {
	return r.DoesRefExist(fmt.Sprintf("refs/remotes/origin/%s", branch))
}

func (r *Repo) DoesRefExist(ref string) (bool, error) {
	out, err := r.Run(&RunOpts{
		Args: []string{"show-ref", ref},
	})
	if err != nil {
		return false, errors.Errorf("ref %s does not exist: %v", ref, err)
	}
	if len(out.Stdout) > 0 {
		return true, nil
	}
	return false, nil
}

func (r *Repo) LsRemote(remote string) (map[string]string, error) {
	out, err := r.Run(&RunOpts{
		Args:      []string{"ls-remote", remote},
		ExitError: true,
	})
	if err != nil {
		return nil, errors.Errorf("failed to get remote branches: %v", err)
	}
	ret := make(map[string]string)
	for _, line := range out.Lines() {
		ss := strings.Split(line, "\t")
		if len(ss) != 2 {
			return nil, errors.Errorf("failed to parse the ls-remote output: %q", line)
		}
		ret[ss[1]] = ss[0]
	}
	return ret, nil
}

type CheckoutBranch struct {
	// The name of the branch to checkout.
	Name string
	// Specifies the "-b" flag to git.
	// The checkout will fail if the branch already exists.
	NewBranch bool
	// Specifies the ref that new branch will have HEAD at
	// Requires the "-b" flag to be specified
	NewHeadRef string
}

// CheckoutBranch performs a checkout of the given branch and returns the name
// of the previous branch, if any (this can be used to restore the previous
// branch if necessary). The returned previous branch name may be empty if the
// repo is currently not checked out to a branch (i.e., in detached HEAD state).
func (r *Repo) CheckoutBranch(opts *CheckoutBranch) (string, error) {
	previousBranchName, err := r.CurrentBranchName()
	if err != nil {
		r.log.WithError(err).
			Debug("failed to get current branch name, repo is probably in detached HEAD")
		previousBranchName = ""
	}

	args := []string{"checkout"}
	if opts.NewBranch {
		args = append(args, "-b")
	}
	args = append(args, opts.Name)
	if opts.NewBranch && opts.NewHeadRef != "" {
		args = append(args, opts.NewHeadRef)
	}
	res, err := r.Run(&RunOpts{
		Args: args,
	})
	if err != nil {
		return "", err
	}
	if res.ExitCode != 0 {
		logrus.WithFields(logrus.Fields{
			"stdout": string(res.Stdout),
			"stderr": string(res.Stderr),
		}).Debug("git checkout failed")
		return "", errors.Errorf("failed to checkout branch %q: %s", opts.Name, string(res.Stderr))
	}
	return previousBranchName, nil
}

// Detach detaches to the detached HEAD.
func (r *Repo) Detach() error {
	res, err := r.Run(&RunOpts{
		Args: []string{"switch", "--detach"},
	})
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		logrus.WithFields(logrus.Fields{
			"stdout": string(res.Stdout),
			"stderr": string(res.Stderr),
		}).Debug("git checkout failed")
		return errors.Errorf("failed to switch to the detached HEAD: %s", string(res.Stderr))
	}
	return nil
}

type RevParse struct {
	// The name of the branch to parse.
	// If empty, the current branch is parsed.
	Rev              string
	SymbolicFullName bool
}

func (r *Repo) RevParse(rp *RevParse) (string, error) {
	args := []string{"rev-parse"}
	if rp.SymbolicFullName {
		args = append(args, "--symbolic-full-name")
	}
	args = append(args, rp.Rev)
	return r.Git(args...)
}

type MergeBase struct {
	Revs []string
}

func (r *Repo) MergeBase(mb *MergeBase) (string, error) {
	args := []string{"merge-base"}
	args = append(args, mb.Revs...)
	return r.Git(args...)
}

type UpdateRef struct {
	// The name of the ref (e.g., refs/heads/my-branch).
	Ref string
	// The Git object ID to set the ref to.
	New string
	// Only update the ref if the current value (before the update) is equal to
	// this object ID. Use Missing to only create the ref if it didn't
	// already exists (e.g., to avoid overwriting a branch).
	Old string

	// Create a reflog for this ref change.
	CreateReflog bool
}

// UpdateRef updates the specified ref within the Git repository.
func (r *Repo) UpdateRef(update *UpdateRef) error {
	args := []string{"update-ref", update.Ref, update.New}
	if update.Old != "" {
		args = append(args, update.Old)
	}
	if update.CreateReflog {
		args = append(args, "--create-reflog")
	}
	_, err := r.Git(args...)
	return errors.WrapIff(err, "failed to write ref %q (%s)", update.Ref, ShortSha(update.New))
}

type Origin struct {
	URL *url.URL
	// The URL slug that corresponds to repository.
	// For example, github.com/my-org/my-repo becomes my-org/my-repo.
	RepoSlug string
}

func (r *Repo) Origin() (*Origin, error) {
	// Note: `git remote get-url` gets the "real" URL of the remote (taking
	// `insteadOf` from git config into account) whereas `git config --get ...`
	// does *not*. Not sure if it matters here.
	output, err := r.Run(&RunOpts{
		Args: []string{"remote", "get-url", "origin"},
	})
	if err != nil {
		return nil, err
	}
	if output.ExitCode != 0 {
		if strings.Contains(string(output.Stderr), "No such remote") {
			return nil, ErrRemoteNotFound
		}
		return nil, errors.New("cannot get the remote of the repository")
	}
	origin := strings.TrimSpace(string(output.Stdout))
	if origin == "" {
		return nil, errors.New("origin URL is empty")
	}

	u, err := giturls.Parse(origin)
	if err != nil {
		return nil, errors.WrapIff(err, "failed to parse origin url %q", origin)
	}

	repoSlug := strings.TrimSuffix(u.Path, ".git")
	repoSlug = strings.TrimPrefix(repoSlug, "/")
	return &Origin{
		URL:      u,
		RepoSlug: repoSlug,
	}, nil
}
