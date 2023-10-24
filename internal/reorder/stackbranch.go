package reorder

import (
	"strings"

	"github.com/aviator-co/av/internal/utils/colors"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/spf13/pflag"
)

// StackBranchCmd is a command to create a new branch in a stack.
//
//	stack-branch <branch-name> [--parent <parent-branch-name>] [--trunk <trunk-branch-name>]
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
	// The branch can be rooted at a given commit by appending "@<commit>" to the
	// branch name.
	Trunk string
	// An optional comment to include in the reorder plan for this command.
	Comment string
}

func (b StackBranchCmd) Execute(ctx *Context) error {
	tx := ctx.DB.WriteTx()
	defer tx.Abort()

	branch, _ := tx.Branch(b.Name)
	var parentState meta.BranchState

	// Figure out which commit we need to start this branch at.
	var headCommit string

	if b.Trunk != "" {
		parentState.Name, headCommit, _ = strings.Cut(b.Trunk, "@")
		parentState.Trunk = true
	} else {
		// We assume the parent branch (if not set manually) is the previous branch
		// in the reorder operation.
		if b.Parent == "" {
			b.Parent = ctx.State.Branch
		}
		if b.Parent == "" {
			return ErrInvalidCmd{
				"stack-branch",
				"--parent=<branch> or --trunk=<branch> must be specified when creating the first branch",
			}
		}
		parentState.Name = b.Parent
		var err error

		// We always start child branches at the HEAD of their parents.
		headCommit, err = ctx.Repo.RevParse(&git.RevParse{Rev: b.Parent})
		if err != nil {
			return err
		}
		parentState.Head = headCommit
	}
	branch.Parent = parentState
	tx.SetBranch(branch)

	if headCommit == "" {
		headCommit = branch.Parent.Name
	}
	if _, err := ctx.Repo.Git("switch", "--force-create", b.Name); err != nil {
		return err
	}
	if _, err := ctx.Repo.Git("reset", "--hard", headCommit); err != nil {
		return err
	}
	ctx.Print(
		"Starting branch ",
		colors.UserInput(b.Name),
		" at ",
		colors.UserInput(git.ShortSha(headCommit)),
		"\n",
	)

	return tx.Commit()
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
	if b.Comment != "" {
		sb.WriteString("  # ")
		sb.WriteString(b.Comment)
	}
	return sb.String()
}

var _ Cmd = &StackBranchCmd{}

func parseStackBranchCmd(args []string) (Cmd, error) {
	cmd := StackBranchCmd{}
	fs := pflag.NewFlagSet("stack-branch", pflag.ContinueOnError)
	fs.StringVar(&cmd.Parent, "parent", "", "parent branch")
	fs.StringVar(&cmd.Trunk, "trunk", "", "trunk branch")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() != 1 {
		return nil, ErrInvalidCmd{
			"stack-branch",
			"exactly one argument is required (the name of the branch to create)",
		}
	}
	if cmd.Trunk != "" && cmd.Parent != "" {
		return nil, ErrInvalidCmd{"stack-branch", "cannot specify both --parent and --trunk"}
	}
	cmd.Name = fs.Arg(0)
	return cmd, nil
}
