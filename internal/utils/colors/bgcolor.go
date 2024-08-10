package colors

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SetupBackgroundColorTypeFromEnv initializes the background color setting based on
// AV_HAS_LIGHT_BG environment variable.
//
// Technically, if terminal sets COLORFGBG environment variable, lipgloss will use it to determine
// if the background color is darker or lighter, but this doesn't necessarily work always, so we
// provide a way to force the background color type.
func SetupBackgroundColorTypeFromEnv() {
	envvar := strings.ToLower(os.Getenv("AV_HAS_LIGHT_BG"))
	switch envvar {
	case "true", "1", "yes", "y", "on":
		lipgloss.SetHasDarkBackground(false)
	case "false", "0", "no", "n", "off":
		lipgloss.SetHasDarkBackground(true)
	default:
		// Otherwise, let lipgloss determine the background color based on the terminal.
	}
	// Workaround for lipgloss / MacOS Terminal.app issue.
	//
	// Inside HasDarkBackground() function, it'll eventually use termStatusReport(11) to get the
	// current terminal background color.
	//
	// https://github.com/muesli/termenv/blob/98d742f6907a4622ef2e2f190123c86b6ec19b7b/termenv_unix.go#L95
	//
	// There are multiple locks in place until this call, but somehow if this is called in the
	// Bubbletea loop, it'll deadlock / hang the program. So, we call it here before the loop
	// starts, and once this is called, it'll be cached, so it'll never be called again.
	lipgloss.HasDarkBackground()
}
