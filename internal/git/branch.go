package git

import (
	"context"
	"fmt"
)

// BranchDelete deletes the given branches (equivalent to `git branch -D`).
func (r *Repo) BranchDelete(ctx context.Context, names ...string) error {
	_, err := r.Run(ctx, &RunOpts{
		Args:      append([]string{"branch", "-D"}, names...),
		ExitError: true,
	})
	return err
}

// BranchSetConfig sets a config on the given branch (equivalent to `git config
// branch.<branch>.<key> <value>`).
func (r *Repo) BranchSetConfig(ctx context.Context, name, key, value string) error {
	_, err := r.Run(ctx, &RunOpts{
		Args:      []string{"config", fmt.Sprintf("branch.%s.%s", name, key), value},
		ExitError: true,
	})
	return err
}

// BranchSetUpstream sets the upstream tracking branch (equivalent to `git branch
// --set-upstream-to=<remote>/<branch> <branch>`).
func (r *Repo) BranchSetUpstream(ctx context.Context, branch, remote string) error {
	_, err := r.Run(ctx, &RunOpts{
		Args:      []string{"branch", "--set-upstream-to", fmt.Sprintf("%s/%s", remote, branch), branch},
		ExitError: true,
	})
	return err
}
