package reorder

import "fmt"

type ErrInvalidCmd struct {
	Cmd    string
	Reason string
}

func (e ErrInvalidCmd) Error() string {
	return fmt.Sprintf("invalid %s command: %s", e.Cmd, e.Reason)
}
