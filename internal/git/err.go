package git

import (
	"os/exec"
	"strings"

	"emperror.dev/errors"
)

func StderrMatches(err error, target string) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return strings.Contains(string(exitErr.Stderr), target)
	}
	return false
}
