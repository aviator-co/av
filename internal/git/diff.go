package git

import (
	"os/exec"

	"emperror.dev/errors"
)

type DiffOpts struct {
	// If specified, generate the diff between the working tree and this commit.
	// If empty (default), generates the diff between the working tree and the
	// current index (i.e., the diff containing all unstaged changes).
	Commit string
	// If true, don't actually generate the diff, just return whether or not its
	// empty. If set, Diff.Contents will always be an empty string.
	Quiet bool
	// If true, shows the colored diff.
	Color bool
	// Both branches need to be specified in order to find the diff between the two branches.
	// If a Commit is specified, the branches will not be used.
	Branch1 string
	Branch2 string
}

type Diff struct {
	// If true, there are no differences between the working tree and the commit.
	Empty    bool
	Contents string
}

func (r *Repo) Diff(d *DiffOpts) (*Diff, error) {
	args := []string{"diff", "--exit-code"}
	if d.Quiet {
		args = append(args, "--quiet")
	}
	if d.Commit != "" {
		args = append(args, d.Commit)
	} else if d.Branch1 != "" && d.Branch2 != "" {
		args = append(args, d.Branch1)
		args = append(args, d.Branch2)
	}

	if d.Color {
		args = append(args, "--color=always")
	}
	contents, err := r.Git(args...)
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
		return &Diff{Empty: false, Contents: contents}, nil
	} else if err != nil {
		return nil, err
	}
	return &Diff{Empty: true, Contents: contents}, nil
}
