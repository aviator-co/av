package uiutils

import (
	"os"
	"strings"

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
		// Use a non-nil empty reader instead of nil. Bubbletea v1.3.10
		// treats nil input as "use os.Stdin", which causes a nil pointer
		// dereference in cancelreader when stdin is /dev/null or a pipe.
		opts = []tea.ProgramOption{
			tea.WithInput(strings.NewReader("")),
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
