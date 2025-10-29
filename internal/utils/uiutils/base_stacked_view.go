package uiutils

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type BaseStackedView struct {
	views []tea.Model
	Err   error
}

func (vm *BaseStackedView) Init() tea.Cmd {
	return nil
}

func (vm *BaseStackedView) Update(msg tea.Msg) tea.Cmd {
	// NOTE: This method is a helper and does not have the signature required
	// by tea.Model.Update. An embedding view model should call this method
	// in its own Update, and return itself as the model. This method only
	// handles updating the internal view stack and returns a command.
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return tea.Quit
		}
	case SimpleCommandMsg:
		return msg.Cmd
	case error:
		vm.Err = msg
		return tea.Quit
	}
	if len(vm.views) > 0 {
		idx := len(vm.views) - 1
		var cmd tea.Cmd
		vm.views[idx], cmd = vm.views[idx].Update(msg)
		return cmd
	}
	return nil
}

func (vm *BaseStackedView) View() string {
	var ss []string
	for _, v := range vm.views {
		r := v.View()
		if r != "" {
			ss = append(ss, r)
		}
	}

	var ret string
	if len(ss) != 0 {
		ret = lipgloss.NewStyle().MarginTop(1).MarginBottom(1).MarginLeft(2).Render(lipgloss.JoinVertical(0, ss...))
	}
	if vm.Err != nil {
		if len(ret) != 0 {
			ret += "\n"
		}
		ret += RenderError(vm.Err)
	}
	if len(ret) > 0 && ret[len(ret)-1] != '\n' {
		ret += "\n"
	}
	return ret
}

func (vm *BaseStackedView) AddView(m tea.Model) tea.Cmd {
	vm.views = append(vm.views, m)
	return m.Init()
}
