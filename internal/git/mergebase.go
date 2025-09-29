package git

import "context"

func (r *Repo) IsAncestor(ctx context.Context, ancestor, descendant string) (bool, error) {
	out, err := r.Run(ctx, &RunOpts{
		Args: []string{"merge-base", "--is-ancestor", ancestor, descendant},
	})
	if err != nil {
		return false, err
	}
	if out.ExitCode != 0 && out.ExitCode != 1 {
		return false, err
	}
	return out.ExitCode == 0, nil
}
