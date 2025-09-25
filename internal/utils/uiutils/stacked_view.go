package uiutils

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type StackedView struct {
	models []tea.Model
}

type stackedViewMsg struct {
	index int
	msg   tea.Msg
}

func NewStackedView(models ...tea.Model) *StackedView {
	return &StackedView{models: models}
}

func (s *StackedView) Push(model tea.Model) tea.Cmd {
	s.models = append(s.models, model)
	cmd := model.Init()
	if cmd == nil {
		return nil
	}
	idx := len(s.models) - 1
	return func() tea.Msg {
		return stackedViewMsg{
			index: idx,
			msg:   cmd(),
		}
	}
}

func (s *StackedView) Init() tea.Cmd {
	var cmds []tea.Cmd
	for i, model := range s.models {
		cmd := model.Init()
		if cmd != nil {
			idx := i
			cmds = append(cmds, func() tea.Msg {
				return stackedViewMsg{
					index: idx,
					msg:   cmd(),
				}
			})
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (s *StackedView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(s.models) == 0 {
		// No models to update.
		return s, nil
	}
	switch msg := msg.(type) {
	case stackedViewMsg:
		if msg.index < 0 || msg.index >= len(s.models) {
			// Invalid index, ignore the message.
			return s, nil
		}
		updatedModel, cmd := s.models[msg.index].Update(msg.msg)
		s.models[msg.index] = updatedModel
		if cmd == nil {
			return s, nil
		}
		return s, func() tea.Msg {
			return stackedViewMsg{
				index: msg.index,
				msg:   cmd(),
			}
		}
	case tea.KeyMsg:
		// Forward key messages to the topmost model.
		topIndex := len(s.models) - 1
		updatedModel, cmd := s.models[topIndex].Update(msg)
		s.models[topIndex] = updatedModel
		if cmd == nil {
			return s, nil
		}
		return s, func() tea.Msg {
			return stackedViewMsg{
				index: topIndex,
				msg:   cmd(),
			}
		}
	default:
		// Ignore other messages.
		return s, nil
	}
}

func (s *StackedView) View() string {
	var ss []string
	for _, model := range s.models {
		ss = append(ss, model.View())
	}
	var ret string
	if len(ss) != 0 {
		ret = lipgloss.JoinVertical(0, ss...)
	}
	return ret
}
