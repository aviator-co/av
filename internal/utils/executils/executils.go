package executils

import (
	"fmt"
	"strings"
)

// FormatCommandLine formats a command line for display.
// This is meant to prevent confusing output when a command line contains
// arguments with spaces or other special characters.
func FormatCommandLine(args []string) string {
	// NB: strings.Builder never returns an error while writing, so we suppress
	// the error return values below.
	sb := strings.Builder{}
	for i, arg := range args {
		if i > 0 {
			_, _ = sb.WriteString(" ")
		}
		if cliArgumentNeedsQuoting(arg) {
			_, _ = fmt.Fprintf(&sb, "%q", arg)
		} else {
			_, _ = sb.WriteString(arg)
		}
	}
	return sb.String()
}

func cliArgumentNeedsQuoting(arg string) bool {
	if arg == "" {
		return true
	}
	for _, r := range arg {
		isAllowedRune := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			(r == '-')
		if !isAllowedRune {
			return true
		}
	}
	return false
}
