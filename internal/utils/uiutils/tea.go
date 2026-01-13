package uiutils

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
)

type BubbleTeaModelWithExitHandling interface {
	// ExitError is called after finish running the program (tea.Quit).
	//
	// This is used as a return value of RunBubbleTea.
	ExitError() error

	tea.Model
}

func RunBubbleTea(model BubbleTeaModelWithExitHandling) error {
	var opts []tea.ProgramOption
	if !isatty.IsTerminal(os.Stdin.Fd()) || !isatty.IsTerminal(os.Stdout.Fd()) {
		opts = []tea.ProgramOption{
			tea.WithInput(nil),
		}
	}
	p := tea.NewProgram(model, opts...)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if err := finalModel.(BubbleTeaModelWithExitHandling).ExitError(); err != nil {
		return err
	}
	return nil
}
