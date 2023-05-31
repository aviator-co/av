package reorder

import (
	"encoding/json"
	"fmt"
	"github.com/aviator-co/av/internal/meta"
	"io"

	"github.com/aviator-co/av/internal/git"
)

// Context is the context of a reorder operation.
// Commands can use the context to access the current state of the reorder.
type Context struct {
	// Repo is the repository the reorder operation is being performed on.
	Repo *git.Repo
	// DB is the av database of the repository.
	DB meta.DB
	// State is the current state of the reorder operation.
	State *State
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
	Head string `json:"head"`
	// The name of the current branch in the reorder operation.
	Branch string `json:"branch"`
	// The sequence of commands to be executed.
	// NOTE: we handle marshalling/unmarshalling in the MarshalJSON/UnmarshalJSON methods.
	Commands []Cmd `json:"-"`
}

func (s *State) MarshalJSON() ([]byte, error) {
	// Create Alias type to avoid copying MarshalJSON method (and avoid infinite recursion).
	type Alias State
	var cmdStrings []string
	for _, cmd := range s.Commands {
		cmdStrings = append(cmdStrings, cmd.String())
	}
	return json.Marshal(&struct {
		Commands []string `json:"commands"`
		*Alias
	}{
		Commands: cmdStrings,
		Alias:    (*Alias)(s),
	})
}

func (s *State) UnmarshalJSON(data []byte) error {
	// Create Alias type to avoid copying UnmarshalJSON method (and avoid infinite recursion).
	type Alias State
	var aux struct {
		Commands []string `json:"commands"`
		Alias
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	var cmds []Cmd
	for _, cmdStr := range aux.Commands {
		cmd, err := ParseCmd(cmdStr)
		if err != nil {
			return err
		}
		cmds = append(cmds, cmd)
	}
	*s = State(aux.Alias)
	s.Commands = cmds
	return nil
}

type Cmd interface {
	// Execute executes the command.
	Execute(ctx *Context) error
	// String returns a string representation of the command.
	// The string representation must be parseable such that
	// ParseCmd(cmd.String()) == cmd.
	String() string
}
