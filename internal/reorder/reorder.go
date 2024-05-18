package reorder

import (
	"fmt"
	"os"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/utils/colors"
)

// Reorder executes a reorder.
// If the reorder couldn't be completed (due to a conflict), a continuation is returned.
// If the reorder was completed successfully, a nil continuation and nil error is returned.
func Reorder(ctx Context) (*State, error) {
	if ctx.Output == nil {
		ctx.Output = os.Stderr
	}

	for _, cmd := range ctx.State.Commands {
		err := cmd.Execute(&ctx)
		if errors.Is(err, ErrInterruptReorder) {
			return ctx.State, nil
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
