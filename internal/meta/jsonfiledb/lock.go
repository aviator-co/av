package jsonfiledb

import (
	"emperror.dev/errors"
	"github.com/gofrs/flock"
)

// lockFile takes an exclusive advisory lock on "<path>.lock" and returns the
// function that releases it. The lock works across processes on Linux, macOS
// and Windows.
func lockFile(path string) (func(), error) {
	l := flock.New(path + ".lock")
	if err := l.Lock(); err != nil {
		return nil, errors.WrapIff(err, "failed to lock av state file %q", path)
	}
	return func() { _ = l.Unlock() }, nil
}
