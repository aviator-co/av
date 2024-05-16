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
}
