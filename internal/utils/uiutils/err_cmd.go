package uiutils

import tea "github.com/charmbracelet/bubbletea"

// ErrCmd wraps an error into a tea.Cmd that returns the error as a message.
//
// Throughout the application, we capture error as a message and use it as a mean to halt the
// application. This is a convenience function to trigger that behavior.
func ErrCmd(err error) tea.Cmd {
	return func() tea.Msg {
		return err
	}
}
