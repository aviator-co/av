package git

import (
	"context"
	"path/filepath"
	"strings"
)

// WorktreeForBranch returns the worktree path where the given branch is checked out,
// or an empty string if the branch is not checked out in any worktree.
// The branch should be in short format (e.g., "my-branch").
func (r *Repo) WorktreeForBranch(ctx context.Context, branch string) (string, error) {
	out, err := r.Run(ctx, &RunOpts{
		Args:      []string{"worktree", "list", "--porcelain"},
		ExitError: true,
	})
	if err != nil {
		return "", err
	}

	repoDir, _ := filepath.EvalSymlinks(r.repoDir)
	var currentWorktree string
	for line := range strings.SplitSeq(string(out.Stdout), "\n") {
		if path, ok := strings.CutPrefix(line, "worktree "); ok {
			currentWorktree = path
		}
		if ref, ok := strings.CutPrefix(line, "branch "); ok {
			shortName := strings.TrimPrefix(ref, "refs/heads/")
			if shortName == branch {
				resolved, _ := filepath.EvalSymlinks(currentWorktree)
				if resolved != repoDir {
					return currentWorktree, nil
				}
			}
		}
	}
	return "", nil
}
