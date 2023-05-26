package reorder

import (
	"fmt"

	"emperror.dev/errors"
)

type ErrInvalidCmd struct {
	Cmd    string
	Reason string
}

func (e ErrInvalidCmd) Error() string {
	return fmt.Sprintf("invalid %s command: %s", e.Cmd, e.Reason)
}

// ErrInterruptReorder is an error that is returned by Cmd implementations when
// the reorder operation should be suspended (and later resumed with --continue,
// --skip, or --reorder).
var ErrInterruptReorder = errors.Sentinel("interrupt reorder")
