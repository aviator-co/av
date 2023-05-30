package reorder

import (
	"fmt"
	"strings"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/errutils"
	"github.com/kr/text"
)

// PickCmd is a command that picks a commit from the history and applies it on
// top of the current HEAD.
type PickCmd struct {
	Commit string
}

func (p PickCmd) Execute(ctx *Context) error {
	err := ctx.Repo.CherryPick(git.CherryPick{
		Commits: []string{p.Commit},
		// Use FastForward to avoid always amending commits.
		FastForward: true,
	})
	if conflict, ok := errutils.As[git.ErrCherryPickConflict](err); ok {
		ctx.Print(
			colors.Failure("  - ", conflict.Error(), "\n"),
			colors.Faint(text.Indent(strings.TrimRight(conflict.Output, "\n"), "        "), "\n"),
		)
		return ErrInterruptReorder
	} else if err != nil {
		return err
	}

	head, err := ctx.Repo.RevParse(&git.RevParse{Rev: "HEAD"})
	if err != nil {
		return err
	}
	ctx.Print(
		colors.Success("  - applied commit "),
		colors.UserInput(git.ShortSha(p.Commit)),
		colors.Success(" without conflict (HEAD is now at "),
		colors.UserInput(git.ShortSha(head)),
		colors.Success(")\n"),
	)
	ctx.State.Head = head
	return nil
}

func (p PickCmd) String() string {
	return fmt.Sprintf("pick %s", p.Commit)
}

var _ Cmd = &PickCmd{}

func parsePickCmd(args []string) (Cmd, error) {
	if len(args) != 1 {
		return nil, ErrInvalidCmd{"pick", "exactly one argument is required (the commit to pick)"}
	}
	return PickCmd{
		Commit: args[0],
	}, nil
}
