package git

// BranchDelete deletes the given branches (equivalent to `git branch -D`).
func (r *Repo) BranchDelete(names ...string) error {
	_, err := r.Run(&RunOpts{
		Args:      append([]string{"branch", "-D"}, names...),
		ExitError: true,
	})
	return err
}
