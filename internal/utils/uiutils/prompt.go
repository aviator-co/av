package uiutils

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/erikgeiser/promptkit/selection"
)

func NewLegacyPromptModel(title string, items []string) *selection.Model[string] {
	ret := selection.NewModel(selection.New(title, items))
	ret.Filter = nil
	ret.KeyMap.Up = append(ret.KeyMap.Up, "k", "ctrl+p")
	ret.KeyMap.Down = append(ret.KeyMap.Down, "j", "ctrl+n")
	return ret
}

var PromptKeys = []key.Binding{
	key.NewBinding(
		key.WithKeys("up", "k", "ctrl+p"),
		key.WithHelp("↑/k", "move up"),
	),
	key.NewBinding(
		key.WithKeys("down", "j", "ctrl+n"),
		key.WithHelp("↓/j", "move down"),
	),
	key.NewBinding(
		key.WithKeys("space", "enter"),
		key.WithHelp("space/enter", "select"),
	),
	key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "cancel"),
	),
}

type PromptModel struct {
	prompt   *selection.Model[string]
	help     help.Model
	callback func(string) tea.Cmd
	quitting bool
}

func NewPromptModel(title string, items []string, callback func(string) tea.Cmd) *PromptModel {
	return &PromptModel{
		prompt:   NewLegacyPromptModel(title, items),
		help:     help.New(),
		callback: callback,
	}
}

func (m *PromptModel) Init() tea.Cmd {
	return m.prompt.Init()
}

func (m *PromptModel) View() string {
	// The base prompt has a new line. Leave the caller to decide whether to add a new line
	// after.
	ret := strings.TrimSpace(m.prompt.View())
	if !m.quitting {
		// Do not show help after finishing the selection.
		ret = lipgloss.JoinVertical(lipgloss.Top, ret, m.help.ShortHelpView(PromptKeys))
	}
	return ret
}

func (m *PromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case " ", "enter":
			m.prompt.Update(msg)
			m.quitting = true
			c, err := m.prompt.Value()
			if err != nil {
				return m, ErrCmd(err)
			}
			return m, m.callback(c)
		case "ctrl+c":
			return m, tea.Quit
		default:
			_, cmd := m.prompt.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}
