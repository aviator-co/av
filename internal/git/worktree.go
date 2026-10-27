package git

import (
	"context"
	"os/exec"
	"strings"

	"emperror.dev/errors"
)

type WorktreeInfo struct {
	Path   string
	HEAD   string
	Branch string
}

func (r *Repo) WorktreeList(ctx context.Context) ([]WorktreeInfo, error) {
	out, err := r.Git(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}

	var worktrees []WorktreeInfo
	var current WorktreeInfo
	for line := range strings.SplitSeq(out, "\n") {
		if line == "" {
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = WorktreeInfo{}
			continue
		}
		if rest, ok := strings.CutPrefix(line, "worktree "); ok {
			current.Path = rest
		} else if rest, ok := strings.CutPrefix(line, "HEAD "); ok {
			current.HEAD = rest
		} else if rest, ok := strings.CutPrefix(line, "branch "); ok {
			current.Branch = strings.TrimPrefix(rest, "refs/heads/")
		}
	}
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}
	return worktrees, nil
}

func DetachWorktreeHEAD(ctx context.Context, worktreePath string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "checkout", "--detach")
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.Errorf("failed to detach HEAD in %s: %s", worktreePath, string(out))
	}
	return nil
}

func RestoreWorktreeBranch(ctx context.Context, worktreePath, branch string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "checkout", branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.Errorf("failed to checkout %s in %s: %s", branch, worktreePath, string(out))
	}
	return nil
}

func IsWorktreeClean(ctx context.Context, worktreePath string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false, errors.Errorf("failed to check worktree status in %s: %v", worktreePath, err)
	}
	return strings.TrimSpace(string(out)) == "", nil
}
