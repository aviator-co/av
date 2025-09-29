package uiutils

import tea "github.com/charmbracelet/bubbletea"

type NewlineModel struct {
	Model tea.Model
}

func (m *NewlineModel) Init() tea.Cmd {
	return m.Model.Init()
}

func (m *NewlineModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	newModel, cmd := m.Model.Update(msg)
	m.Model = newModel
	return m, cmd
}

func (m *NewlineModel) View() string {
	return m.Model.View() + "\n"
}
