package reorder

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/errutils"
)

type DeleteBranchMode string

// DeleteBranchCmd is a command that deletes a branch.
type DeleteBranchCmd struct {
	Name string
	// If true, delete the branch from Git as well as from the internal database.
	// If false, only delete the branch metadata from the internal database.
	DeleteRef bool
}

func (d DeleteBranchCmd) Execute(ctx *Context) error {
	tx := ctx.DB.WriteTx()
	tx.DeleteBranch(d.Name)
	if err := tx.Commit(); err != nil {
		return err
	}

	if !d.DeleteRef {
		_, _ = fmt.Fprint(os.Stderr,
			"Orphaned branch ", colors.UserInput(d.Name), ".\n",
			"  - Run ", colors.CliCmd("git branch --delete ", d.Name),
			" to delete the branch from your repository.\n",
		)
		return nil
	}

	_, err := ctx.Repo.Run(&git.RunOpts{
		Args:      []string{"branch", "--delete", "--force", d.Name},
		ExitError: true,
	})
	if exiterr, ok := errutils.As[*exec.ExitError](err); ok &&
		strings.Contains(string(exiterr.Stderr), "not found") {
		_, _ = fmt.Fprint(os.Stderr,
			colors.Warning("Branch "), colors.UserInput(d.Name),
			colors.Warning(" was already deleted.\n"),
		)
		return nil
	} else if err != nil {
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
	if d.DeleteRef {
		sb.WriteString(" --delete-ref")
	}
	return sb.String()
}

var _ Cmd = DeleteBranchCmd{}
