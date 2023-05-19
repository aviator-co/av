package reorder

import (
	"fmt"
)

// PickCmd is a command that picks a commit from the history and applies it on
// top of the current HEAD.
type PickCmd struct {
	Commit string
}

func (b PickCmd) Execute(ctx *Context) error {
	panic("not implemented")
}

func (b PickCmd) String() string {
	return fmt.Sprintf("pick %s", b.Commit)
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
