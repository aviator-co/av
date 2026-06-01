package colors

import (
	"os"
	"strings"

	"charm.land/lipgloss/v2"
)

// hasDarkBackground is initialized once by SetupBackgroundColorTypeFromEnv
// before the Bubble Tea loop starts and read via HasDarkBackground.
var hasDarkBackground = true

// SetupBackgroundColorTypeFromEnv determines whether the terminal has a dark
// background and caches it in hasDarkBackground. Terminal detection isn't always
// reliable, so AV_HAS_LIGHT_BG can force the result.
func SetupBackgroundColorTypeFromEnv() {
	envvar := strings.ToLower(os.Getenv("AV_HAS_LIGHT_BG"))
	switch envvar {
	case "true", "1", "yes", "y", "on":
		hasDarkBackground = false
	case "false", "0", "no", "n", "off":
		hasDarkBackground = true
	default:
		// Query the terminal once, here, before any Bubble Tea program runs.
		// lipgloss.HasDarkBackground does synchronous terminal I/O (raw-mode stdin,
		// an OSC 11 write to stdout, then reads the reply), so it must not run while
		// a program owns the terminal, and RenderError consumes the result from
		// inside a View. v2's lipgloss doesn't cache it, so we store it ourselves.
		hasDarkBackground = lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
	}
}

// HasDarkBackground reports whether the terminal has a dark background, as
// determined by SetupBackgroundColorTypeFromEnv.
func HasDarkBackground() bool {
	return hasDarkBackground
}
