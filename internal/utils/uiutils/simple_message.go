package uiutils

import tea "charm.land/bubbletea/v2"

type SimpleMessageView struct {
	Message string
}

func (s SimpleMessageView) View() tea.View {
	return tea.NewView(s.Message)
}

func (s SimpleMessageView) Init() tea.Cmd {
	return nil
}

func (s SimpleMessageView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return s, nil
}
