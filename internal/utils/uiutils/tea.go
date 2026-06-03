package uiutils

import (
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	// Handle input and output independently: the renderer writes to stdout, while
	// input is read from stdin.
	stdinIsTTY := isatty.IsTerminal(os.Stdin.Fd())
	stdoutIsTTY := isatty.IsTerminal(os.Stdout.Fd())
	var opts []tea.ProgramOption
	if !stdinIsTTY {
		// Empty reader instead of nil so the program doesn't try to open
		// /dev/tty for input, which fails ("could not open TTY") under pipes/CI.
		opts = append(opts, tea.WithInput(strings.NewReader("")))
	}
	if !stdoutIsTTY {
		// Bubbletea v2's renderer is built for interactive terminals; with no
		// terminal it can't size a frame and emits control sequences with no
		// content. Skip it and print the final view ourselves so piped/redirected
		// output stays clean and complete.
		opts = append(opts, tea.WithoutRenderer())
	}
	p := tea.NewProgram(model, opts...)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if !stdoutIsTTY {
		// lipgloss v2 doesn't strip color on its own, and we bypassed bubbletea's
		// renderer, so use lipgloss's fmt drop-in, which downsamples color to the
		// output's profile (stripping ANSI entirely for non-terminals).
		_, _ = lipgloss.Fprint(os.Stdout, finalModel.View().Content)
	}
	if err := finalModel.(BubbleTeaModelWithExitHandling).ExitError(); err != nil {
		return err
	}
	return nil
}
