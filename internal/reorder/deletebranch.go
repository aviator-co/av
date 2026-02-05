package reorder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/pflag"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/errutils"
)

// DeleteBranchCmd is a command that deletes a branch.
// This is for internal use only to clean up branches that were removed from a
// re-order operation.
type DeleteBranchCmd struct {
	Name string
	// If true, delete the branch from Git as well as from the internal database.
	// If false, only delete the branch metadata from the internal database.
	DeleteGitRef bool
}

func (d DeleteBranchCmd) Execute(ctx *Context) error {
	tx := ctx.DB.WriteTx()
	tx.DeleteBranch(d.Name)
	if err := tx.Commit(); err != nil {
		return err
	}

	if !d.DeleteGitRef {
		_, _ = fmt.Fprint(os.Stderr,
			"Orphaned branch ", colors.UserInput(d.Name), ".\n",
			"  - Run ", colors.CliCmd("git branch --delete ", d.Name),
			" to delete the branch from your repository.\n",
		)
		return nil
	}

	_, err := ctx.Repo.Run(context.Background(), &git.RunOpts{
		Args:      []string{"branch", "--delete", "--force", d.Name},
		ExitError: true,
	})
	if exiterr, ok := errutils.As[*exec.ExitError](err); ok {
		stderr := string(exiterr.Stderr)
		if strings.Contains(stderr, "not found") {
			_, _ = fmt.Fprint(os.Stderr,
				colors.Warning("Branch "), colors.UserInput(d.Name),
				colors.Warning(" was already deleted.\n"),
			)
			return nil
		}
		if strings.Contains(stderr, "used by worktree") {
			msg := fmt.Sprintf("cannot delete branch %q: it is checked out in a worktree\n", d.Name)
			msg += "Use 'git worktree list' to see all worktrees\n"
			msg += "Remove the worktree with 'git worktree remove <path>' or checkout a different branch in that worktree\n"
			_, _ = fmt.Fprint(os.Stderr, msg)
			return nil
		}
	}
	if err != nil {
		return err
	}

	_, _ = fmt.Fprint(os.Stderr,
		"Deleted branch ", colors.UserInput(d.Name), ".\n",
	)
	return nil
}

func (d DeleteBranchCmd) String() string {
	sb := strings.Builder{}
	sb.WriteString("delete-branch ")
	sb.WriteString(d.Name)
	if d.DeleteGitRef {
		sb.WriteString(" --delete-git-ref")
	}
	return sb.String()
}

var _ Cmd = DeleteBranchCmd{}

func parseDeleteBranchCmd(args []string) (Cmd, error) {
	cmd := DeleteBranchCmd{}
	flags := pflag.NewFlagSet("delete-branch", pflag.ContinueOnError)
	flags.BoolVar(
		&cmd.DeleteGitRef,
		"delete-git-ref",
		false,
		"delete the branch from Git as well as from the internal database",
	)
	if err := flags.Parse(args); err != nil {
		return nil, err
	}
	if flags.NArg() != 1 {
		return nil, ErrInvalidCmd{
			"delete-branch",
			"exactly one argument is required (the name of the branch to delete)",
		}
	}
	cmd.Name = flags.Arg(0)
	return cmd, nil
}
