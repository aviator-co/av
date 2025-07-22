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
