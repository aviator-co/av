package reorder

import (
	"emperror.dev/errors"
	"github.com/google/shlex"
)

// ParseCmd parses a reorder command from a string.
// Comments must be stripped from the input before calling this function.
func ParseCmd(line string) (Cmd, error) {
	args, err := shlex.Split(line)
	if err != nil {
		return nil, errors.Wrap(err, "invalid reorder command")
	}
	if len(args) == 0 {
		return nil, errors.New("empty reorder command")
	}
	cmdName := args[0]
	args = args[1:]
	switch cmdName {
	case "delete-branch", "db":
		return parseDeleteBranchCmd(args)
	case "pick", "p":
		return parsePickCmd(args)
	case "stack-branch", "sb":
		return parseStackBranchCmd(args)
	default:
		return nil, errors.Errorf("unknown reorder command %q", cmdName)
	}
}
