package git

import (
	"bytes"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"emperror.dev/errors"
    "github.com/aviator-co/av/internal/utils/stringutils"
	"github.com/sirupsen/logrus"
	giturls "github.com/whilp/git-urls"
)

type Repo struct {
	repoDir string
	log     logrus.FieldLogger
}

func OpenRepo(repoDir string) (*Repo, error) {
	r := &Repo{
		repoDir,
		logrus.WithFields(logrus.Fields{"repo": path.Base(repoDir)}),
	}

	return r, nil
}

func (r *Repo) Dir() string {
	return r.repoDir
}

func (r *Repo) GitDir() string {
	return path.Join(r.repoDir, ".git")
}

func (r *Repo) DefaultBranch() (string, error) {
    remote, err := r.DefaultRemote()
    if err != nil {
        return "", err
    }
    
	ref, err := r.Git("symbolic-ref", "refs/remotes/" + remote.Label + "/HEAD")
	if err != nil {
		logrus.WithError(err).Debug("failed to determine remote HEAD")
		// this communicates with the remote, so we probably don't want to run
		// it by default, but we helpfully suggest it to the user. :shrug:
		logrus.Warn(
			"Failed to determine repository default branch. " +
				"Ensure you have a remote named " + remote.Label +
				" and try running `git remote set-head --auto " + remote.Label + "` to fix this.",
		)
		return "", errors.New("failed to determine remote HEAD")
	}
	return strings.TrimPrefix(ref, "refs/remotes/" + remote.Label + "/"), nil
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
}

type Output struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

func (o Output) Lines() []string {
	s := strings.TrimSpace(string(o.Stdout))
	return strings.Split(s, "\n")
}

func (r *Repo) Run(opts *RunOpts) (*Output, error) {
	cmd := exec.Command("git", opts.Args...)
	cmd.Dir = r.repoDir
	r.log.Debugf("git %s", opts.Args)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), opts.Env...)
	err := cmd.Run()
	var exitError *exec.ExitError
	if err != nil && !errors.As(err, &exitError) {
		return nil, errors.Wrapf(err, "git %s", opts.Args)
	}
	if err != nil && opts.ExitError && exitError.ExitCode() != 0 {
		return nil, errors.Errorf("git %s: %s: %s", opts.Args, err, stderr.String())
	}
	return &Output{
		ExitCode: cmd.ProcessState.ExitCode(),
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
	}, nil
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
// IMPORTANT: This function will return an error if the repository is currently
// in a detached-head state (e.g., during a rebase conflict).
func (r *Repo) CurrentBranchName() (string, error) {
	branch, err := r.Git("symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", errors.Wrap(err, "failed to determine current branch (are you in detached HEAD or is a rebase in progress?)")
	}
	return branch, nil
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

type Remote struct {
	// the label given to the remote config, typically "origin"
    Label string
	URL *url.URL
	// The URL slug that corresponds to repository.
	// For example, github.com/my-org/my-repo becomes my-org/my-repo.
	RepoSlug string
}

func getRemote(repoDir string, label string) (*Remote, error) {
    // Note: 'git remote get-url' gets the "real" URL of the remote (taking
    // 'insteadOf' from git config into account) whereas 'git config --get ...'
    // does *not*. Not sure if it matters here.
    cmd := exec.Command("git", "remote", "get-url", label)
    cmd.Dir = repoDir
    out, err := cmd.Output()

	if err != nil {
		return nil, err
	}
	
	remoteUrl := strings.TrimSpace(string(out))
	
	if remoteUrl == "" {
		return nil, errors.New("remote URL is empty")
	}

	u, err := giturls.Parse(remoteUrl)
	if err != nil {
		return nil, errors.WrapIff(err, "failed to parse remote url %q", remoteUrl)
	}

	repoSlug := strings.TrimSuffix(u.Path, ".git")
	repoSlug = strings.TrimPrefix(repoSlug, "/")
	return &Remote{
	    Label:    label,
		URL:      u,
		RepoSlug: repoSlug,
	}, nil
}

type RemoteConfig struct {
    defaultRemote string
    remoteMap map[string]*Remote 
}

func (r *Repo) RemoteConfig() (*RemoteConfig, error) {
    cmd := exec.Command("git", "remote")
    cmd.Dir = r.repoDir
    out, err := cmd.Output()
    
    if err != nil {
        return nil, err
    }
    
    strOut := string(out)
    defaultRemote := ""
    remoteMap := make(map[string]*Remote)
    
    if strOut == "" {
        return nil, errors.New("no remote config found")
    }
    
    remoteLabels := stringutils.SplitLines(strOut)
    
    for i, label := range remoteLabels {
        // use "origin" as default remote if found, otherwise first listed remote
        if i == 0 || label == "origin" {
            defaultRemote = label
        }
        
        remote, err := getRemote(r.repoDir, label)
        
        if err != nil {
            return nil, err
        }
        
        remoteMap[label] = remote
    }
    
    return &RemoteConfig{defaultRemote, remoteMap}, nil
}

func (r *Repo) Remote(label string) (*Remote, error) {
    remoteConfig, err := r.RemoteConfig()
    if err !=nil {
        return nil, err
    }
    
    remote, ok := remoteConfig.remoteMap[label]
    if !ok {
        return nil, errors.New("no remote config found for " + label)
    }
    
    return remote, nil
}

func (r *Repo) DefaultRemote() (*Remote, error) {
    remoteConfig, err := r.RemoteConfig()
    if err !=nil {
        return nil, err
    }
    
    return remoteConfig.remoteMap[remoteConfig.defaultRemote], nil
}
