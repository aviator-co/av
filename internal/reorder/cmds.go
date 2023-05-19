package reorder

// Context is the context of a reorder operation.
// Commands can use the context to access the current state of the reorder
// operation and mutate the context to reflect their changes.
type Context struct {
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
