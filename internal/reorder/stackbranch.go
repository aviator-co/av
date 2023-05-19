package reorder

import (
	"github.com/spf13/pflag"
	"strings"
)

// StackBranchCmd is a command to create a new branch in a stack.
//
//	branch <branch-name> [--parent <parent-branch-name>] [--trunk <trunk-branch-name>]
type StackBranchCmd struct {
	// The name of the branch to create.
	Name string
	// The name of the parent branch. If not specified, the previous branch in
	// the reorder stack is used (or an error is raised if there is no previous
	// branch).
	// Mutually exclusive with --trunk.
	Parent string
	// The name of the trunk branch.
	// Mutually exclusive with --parent.
	Trunk string
}

func (b StackBranchCmd) Execute(ctx *Context) error {
	panic("not implemented")
}

func (b StackBranchCmd) String() string {
	sb := strings.Builder{}
	sb.WriteString("stack-branch ")
	sb.WriteString(b.Name)
	if b.Parent != "" {
		sb.WriteString(" --parent ")
		sb.WriteString(b.Parent)
	}
	if b.Trunk != "" {
		sb.WriteString(" --trunk ")
		sb.WriteString(b.Trunk)
	}
	return sb.String()
}

var _ Cmd = &StackBranchCmd{}

func parseBranchCmd(args []string) (Cmd, error) {
	cmd := StackBranchCmd{}
	fs := pflag.NewFlagSet("stack-branch", pflag.ContinueOnError)
	fs.StringVar(&cmd.Parent, "parent", "", "parent branch")
	fs.StringVar(&cmd.Trunk, "trunk", "", "trunk branch")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() != 1 {
		return nil, ErrInvalidCmd{"branch", "exactly one argument is required (the name of the branch to create)"}
	}
	if cmd.Trunk != "" && cmd.Parent != "" {
		return nil, ErrInvalidCmd{"branch", "cannot specify both --parent and --trunk"}
	}
	cmd.Name = fs.Arg(0)
	return cmd, nil
}
