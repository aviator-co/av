package git

type RebaseOpts struct {
	// Required (unless Continue is true)
	// The upstream branch to rebase onto.
	Upstream string
	// Optional (mutually exclusive with all other options)
	// If set, continue a rebase (all other options are ignored).
	Continue bool
	// Optional
	// If set, use `git rebase --onto <upstream> ...`
	Onto string
	// Optional
	// If set, this is the branch that will be rebased; otherwise, the current
	// branch is rebased.
	Branch string
}

func (r *Repo) Rebase(opts RebaseOpts) (*Output, error) {
	// TODO: probably move the parseRebaseOutput logic in sync to here

	args := []string{"rebase"}
	if opts.Continue {
		return r.Run(&RunOpts{
			Args: []string{"rebase", "--continue"},
			// `git rebase --continue` will open an editor to allow the user
			// to edit the commit message, which we don't want here. Instead, we
			// specify `true` here (which is a command that does nothing and
			// simply exits 0) to disable the editor.
			Env: []string{"GIT_EDITOR=true"},
		})
	}
	if opts.Onto != "" {
		args = append(args, "--onto", opts.Onto)
	}
	args = append(args, opts.Upstream)
	if opts.Branch != "" {
		args = append(args, opts.Branch)
	}

	return r.Run(&RunOpts{Args: args})
}
