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

// SetUpstream configures the upstream tracking relationship for a local branch
// (equivalent to `git branch --set-upstream-to=<remote>/<branch> <branch>`).
func (r *Repo) SetUpstream(ctx context.Context, branchName, remoteName string) error {
	if err := r.BranchSetConfig(ctx, branchName, "remote", remoteName); err != nil {
		return err
	}
	if err := r.BranchSetConfig(ctx, branchName, "merge", fmt.Sprintf("refs/heads/%s", branchName)); err != nil {
		return err
	}
	return nil
}
