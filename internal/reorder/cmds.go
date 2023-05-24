package reorder

import (
	"fmt"
	"github.com/aviator-co/av/internal/git"
	"io"
)

// Context is the context of a reorder operation.
// Commands can use the context to access the current state of the reorder.
type Context struct {
	// Repo is the repository the reorder operation is being performed on.
	Repo *git.Repo
	// State is the current state of the reorder operation.
	State State
	// Output is the output stream to write interactive messages to.
	// Commands should write to this stream instead of stdout/stderr.
	Output io.Writer
}

func (c *Context) Print(a ...any) {
	_, _ = fmt.Fprint(c.Output, a...)
}

// State is the state of a reorder operation.
// It is meant to be serializable to allow the user to continue/abort a reorder
// operation if there is a conflict.
type State struct {
	// The current HEAD of the reorder operation.
	Head string
	// The name of the current branch in the reorder operation.
	Branch string
}

type Cmd interface {
	// Execute executes the command.
	Execute(ctx *Context) error
	// String returns a string representation of the command.
	// The string representation must be parseable such that
	// ParseCmd(cmd.String()) == cmd.
	String() string
}
