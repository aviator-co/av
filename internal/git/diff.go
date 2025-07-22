package git

import (
	"context"

	"emperror.dev/errors"
)

type DiffOpts struct {
	// The revisions to compare.
	// The behavior of the diff changes depending on how these are specified.
	//   - If empty, the generated diff is relative to the current staging area.
	//   - If one commit is given, the diff is calculated between the working tree
	//     and the given commit.
	//   - If two commits are given (or one string representing a commit range
	//     like `<a>..<b>`), the diff is calculated between the two commits.
	Specifiers []string
	// If true, don't actually generate the diff, just return whether or not its
	// empty. If set, Diff.Contents will always be an empty string.
	Quiet bool
	// If true, shows the colored diff.
	Color bool
	// If specified, compare only the specified paths.
	Paths []string
}

type Diff struct {
	// If true, there are no differences between the working tree and the commit.
	Empty    bool
	Contents string
}

func (r *Repo) Diff(ctx context.Context, d *DiffOpts) (*Diff, error) {
	args := []string{"diff", "--exit-code"}
	if d.Quiet {
		args = append(args, "--quiet")
	}
	if d.Color {
		args = append(args, "--color=always")
	}

	args = append(args, d.Specifiers...)

	// This needs to be last because everything after the `--` is interpreted
	// as a path, not a flag.
	// Note that we still append this `--` even if there are no paths because
	// otherwise Git might interpret a specifier as ambiguous path and raise an
	// error.
	args = append(args, "--")
	args = append(args, d.Paths...)

	output, err := r.Run(ctx, &RunOpts{
		Args: args,
	})
	if err != nil {
		return nil, err
	}
	if output.ExitCode == 1 {
		return &Diff{Empty: false, Contents: string(output.Stdout)}, nil
	} else if output.ExitCode != 0 {
		return nil, errors.Errorf("git diff failed: %s", string(output.Stderr))
	}
	return &Diff{Empty: true, Contents: string(output.Stdout)}, nil
}
