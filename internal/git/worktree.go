package git

import (
	"context"
	"strings"
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
