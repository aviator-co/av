package git

import "context"

type RevListOpts struct {
	// A list of commit roots, or exclusions if the commit sha starts with a
	// caret (^). As a special case, "foo..bar" is equivalent to "foo ^bar"
	// which means every commit reachable from foo but not from bar.
	// For example, to list all of the commits introduced in a pull request,
	// the specifier would be "HEAD..master".
	// See `git rev-list --help`.
	Specifiers []string

	// If true, display the commits in chronological order.
	Reverse bool
}

// RevList list commits that are reachable from the given commits (excluding
// commits reachable from the given exclusions).
func (r *Repo) RevList(ctx context.Context, opts RevListOpts) ([]string, error) {
	args := []string{"rev-list"}
	if opts.Reverse {
		args = append(args, "--reverse")
	}
	args = append(args, opts.Specifiers...)
	// Unambiguous the positional arguments
	args = append(args, "--")
	res, err := r.Run(ctx, &RunOpts{
		Args:      args,
		Env:       nil,
		ExitError: true,
	})
	if err != nil {
		return nil, err
	}
	return res.Lines(), nil
}
