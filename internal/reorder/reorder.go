package reorder

import (
	"fmt"
	"os"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/utils/colors"
)

// Reorder executes a reorder.
// If the reorder couldn't be completed (due to a conflict), a non-nil
// *Continuation is returned describing how to resume.
// If the reorder was completed successfully, nil and nil are returned.
func Reorder(ctx Context) (*Continuation, error) {
	if ctx.Output == nil {
		ctx.Output = os.Stderr
	}

	for _, cmd := range ctx.State.Commands {
		err := cmd.Execute(&ctx)
		if errors.Is(err, ErrInterruptReorder) {
			cont := &Continuation{State: ctx.State}
			// If the interrupted command is a squash/fixup, mark the
			// continuation so that --continue knows to fold the commit after
			// the cherry-pick conflict is resolved.
			if pickCmd, ok := cmd.(PickCmd); ok && pickCmd.Mode != PickModePick {
				cont.SquashPending = true
			}
			return cont, nil
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
	// SquashPending is true when the reorder was interrupted mid-squash or
	// mid-fixup (i.e., a cherry-pick conflict occurred while applying a
	// squash/fixup command). In that case, --continue must call PerformSquash
	// after resuming the cherry-pick to fold the commit into its predecessor.
	SquashPending bool
}
