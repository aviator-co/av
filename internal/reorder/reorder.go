package reorder

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aviator-co/av/internal/git"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/utils/colors"
)

// Reorder executes a reorder.
// If the reorder couldn't be completed (due to a conflict), a continuation is returned.
// If the reorder was completed successfully, a nil continuation and nil error is returned.
func Reorder(ctx Context) (*Continuation, error) {
	if ctx.Output == nil {
		ctx.Output = os.Stderr
	}

	for _, cmd := range ctx.State.Commands {
		err := cmd.Execute(&ctx)
		if errors.Is(err, ErrInterruptReorder) {
			return &Continuation{State: ctx.State}, nil
		} else if err != nil {
			return nil, err
		}
		ctx.State.Commands = ctx.State.Commands[1:]
	}

	_, _ = fmt.Fprint(ctx.Output, colors.Success("Reorder complete!\n"))
	return nil, nil
}

type Continuation struct {
	State *State
}

const stateFileName = "stack-reorder.state.json"

// ReadContinuation reads a continuation from the state file.
// Returns the raw error returned by os.Open if the file couldn't be opened.
// Use os.IsNotExist to check if the continuation doesn't exist.
func ReadContinuation(repo *git.Repo) (*Continuation, error) {
	file, err := os.Open(filepath.Join(repo.AvDir(), stateFileName))
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	decoder := json.NewDecoder(file)
	var continuation Continuation
	err = decoder.Decode(&continuation)
	if err != nil {
		return nil, err
	}

	return &continuation, nil
}

// WriteContinuation writes a continuation to the state file.
// If a nil continuation is passed, the state file is deleted.
func WriteContinuation(repo *git.Repo, continuation *Continuation) error {
	if continuation == nil {
		return os.Remove(filepath.Join(repo.AvDir(), stateFileName))
	}

	file, err := os.Create(filepath.Join(repo.AvDir(), stateFileName))
	if err != nil {
		return err
	}
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err := enc.Encode(continuation); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
