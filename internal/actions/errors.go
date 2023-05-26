package actions

import "emperror.dev/errors"

var ErrRepoNotInitialized = errors.Sentinel("this repository is not initialized; please run `av init`")


// errExitSilently is an error type that indicates that program should exit
// without printing any additional information with the given exit code.
// This is meant for cases where the running commands wants to manage its own
// error output but still needs to return a non-zero exit code (since returning
// nil from RunE would cause a exit with a zero code).
type ErrExitSilently struct {
	ExitCode int
}

func (e ErrExitSilently) Error() string {
	return "<exit silently>"
}