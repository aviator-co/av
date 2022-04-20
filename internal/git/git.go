package git

import (
	"emperror.dev/errors"
	"github.com/sirupsen/logrus"
	"io"
	"os/exec"
	"strings"
)

// Missing is a sentinel zero-value for object id (aka sha).
// Git treats this value as "this thing doesn't exist".
// For example, when updating a ref, if the old value is specified as EmptyOid,
// Git will refuse to update the ref if already exists.
const Missing = "0000000000000000000000000000000000000000"

type Repo struct {
	repoDir       string
	defaultBranch string
	log           logrus.FieldLogger
}

func OpenRepo(repoDir string) (*Repo, error) {
	r := &Repo{
		repoDir, "",
		logrus.WithFields(logrus.Fields{"repo": repoDir}),
	}

	for _, possibleTrunk := range []string{"main", "master", "default", "trunk"} {
		_, err := r.Git("rev-parse", "refs/remotes/origin/"+possibleTrunk)
		if err == nil {
			r.defaultBranch = possibleTrunk
			break
		}
	}
	return r, nil
}

func (r *Repo) DefaultBranch() string {
	return r.defaultBranch
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

// CheckoutWithCleanup performs a checkout of the given branch and returns a function that can be
// used to revert the checkout. This is useful when switching to another branch and switching back
// during cleanup.
func (r *Repo) CheckoutWithCleanup(args ...string) (func(), error) {
	currentBranch, err := r.CurrentBranchName()
	if err != nil {
		return nil, errors.WrapIf(err, "not currently on branch")
	}
	args = append([]string{"checkout"}, args...)
	if _, err := r.Git(args...); err != nil {
		return nil, err
	}
	return func() {
		if _, err := r.Git("checkout", currentBranch); err != nil {
			r.log.WithError(err).Error("failed to revert checkout")
		}
	}, nil
}

func (r *Repo) HeadOid() (string, error) {
	return r.Git("rev-parse", "HEAD")
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
