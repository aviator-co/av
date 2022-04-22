package git

import (
	"emperror.dev/errors"
	"github.com/sirupsen/logrus"
	"io"
	"os/exec"
	"path"
	"strings"
)

type Repo struct {
	// immutable
	repoDir string
	log     logrus.FieldLogger

	// mutable (mostly for caching things that are relatively expensive to
	// compute)
	defaultBranch string
}

func OpenRepo(repoDir string) (*Repo, error) {
	r := &Repo{
		repoDir: repoDir,
		log:     logrus.WithFields(logrus.Fields{"repo": path.Base(repoDir)}),
	}

	return r, nil
}

func (r *Repo) DefaultBranch() (string, error) {
	if r.defaultBranch == "" {
		// TODO: we should just cache this in some metadata file somewhere
		// (e.g. .git/av-metadata)
		refs, err := r.Git("ls-remote", "--symref", "origin", "HEAD")
		if err != nil {
			return "", errors.WrapIf(err, "failed to list remote refs")
		}
		for _, ref := range strings.Split(refs, "\n") {
			if !strings.HasPrefix(ref, "refs: ") {
				continue
			}
			ref = strings.TrimPrefix(ref, "refs: ")
			parts := strings.Split(ref, "/")
			r.defaultBranch = parts[len(parts)-1]
		}
	}
	if r.defaultBranch == "" {
		return "", errors.New("failed to determine repository default branch")
	}
	return r.defaultBranch, nil
}

func (r *Repo) Git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.repoDir
	r.log.Debugf("git %s", args)
	out, err := cmd.Output()
	if err != nil {
		stderr := "<no output>"
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			stderr = string(exitError.Stderr)
		}
		r.log.Debugf("git %s failed: %s: %s", args, err, stderr)
		return "", errors.Wrapf(err, "git %s", args[0])
	}

	// trim trailing newline
	return strings.TrimSpace(string(out)), nil
}

func (r *Repo) GitStdin(args []string, stdin io.Reader) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.repoDir
	cmd.Stdin = stdin
	r.log.Debugf("git %s", args)
	out, err := cmd.Output()
	if err != nil {
		stderr := "<no output>"
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			stderr = string(exitError.Stderr)
		}
		r.log.Errorf("git %s failed: %s: %s", args, err, stderr)
		return "", errors.Wrapf(err, "git %s", args[0])
	}

	// trim trailing newline
	return strings.TrimSpace(string(out)), nil
}

// CurrentBranchName returns the name of the current branch.
// The name is return in "short" format -- i.e., without the "refs/heads/" prefix.
func (r *Repo) CurrentBranchName() (string, error) {
	return r.Git("symbolic-ref", "--short", "HEAD")
}

type CheckoutBranch struct {
	// The name of the branch to checkout.
	Name string
	// Specifies the "-b" flag to git.
	// The checkout will fail if the branch already exists.
	NewBranch bool
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
	if _, err := r.Git(args...); err != nil {
		return "", err
	}
	return previousBranchName, nil
}

type RevParse struct {
	// The name of the branch to parse.
	// If empty, the current branch is parsed.
	Rev string
}

func (r *Repo) RevParse(rp *RevParse) (string, error) {
	args := []string{"rev-parse", rp.Rev}
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
}

// UpdateRef updates the specified ref within the Git repository.
func (r *Repo) UpdateRef(update *UpdateRef) error {
	args := []string{"update-ref", update.Ref, update.New}
	if update.Old != "" {
		args = append(args, update.Old)
	}
	_, err := r.Git(args...)
	return errors.WrapIff(err, "failed to write ref %q (%s)", update.Ref, ShortSha(update.New))
}
