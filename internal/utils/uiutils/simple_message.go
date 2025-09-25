package uiutils

import tea "github.com/charmbracelet/bubbletea"

type SimpleMessageView struct {
	Message string
}

func (s SimpleMessageView) View() string {
	return s.Message
}

func (s SimpleMessageView) Init() tea.Cmd {
	return nil
}

func (s SimpleMessageView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return s, nil
}
