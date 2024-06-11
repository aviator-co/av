package uiutils

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/erikgeiser/promptkit/selection"
)

func NewPromptModel(title string, items []string) *selection.Model[string] {
	ret := selection.NewModel(selection.New(title, items))
	ret.Filter = nil
	ret.KeyMap.Up = append(ret.KeyMap.Up, "k")
	ret.KeyMap.Down = append(ret.KeyMap.Down, "j")
	return ret
}

var PromptKeys = []key.Binding{
	key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	key.NewBinding(
		key.WithKeys("down", "j"),
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
