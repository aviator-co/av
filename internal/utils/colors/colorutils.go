package colors

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/fatih/color"
)

var (
	CliCmdC          = color.New(color.FgMagenta)
	SuccessC         = color.New(color.FgGreen)
	WarningC         = color.New(color.FgYellow)
	FailureC         = color.New(color.FgRed)
	TroubleshootingC = color.New(color.Faint)
	UserInputC       = color.New(color.FgCyan)
	FaintC           = color.New(color.Faint)
)

var (
	CliCmd          = CliCmdC.Sprint
	Success         = SuccessC.Sprint
	Warning         = WarningC.Sprint
	Failure         = FailureC.Sprint
	Troubleshooting = TroubleshootingC.Sprint
	UserInput       = UserInputC.Sprint
	Faint           = FaintC.Sprint
)

var (
	ProgressStyle = lipgloss.NewStyle().Foreground(Amber600)
	SuccessStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color('2'))
	FailureStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color('1'))

	// Aligned with promptkit
	// https://github.com/erikgeiser/promptkit/blob/main/selection/prompt.go#L62
	PromptChoice = lipgloss.NewStyle().Foreground(lipgloss.Color("32")).Bold(true)
)
