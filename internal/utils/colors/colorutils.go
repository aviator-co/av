package colors

import "github.com/fatih/color"

var (
	CliCmdC          = color.New(color.FgMagenta)
	SuccessC         = color.New(color.FgGreen)
	WarningC         = color.New(color.FgYellow)
	FailureC         = color.New(color.FgRed)
	TroubleshootingC = color.New(color.Faint)
	UserInputC       = color.New(color.FgCyan)
	FaintC           = color.New(color.Faint)
	BoldC			 = color.New(color.Bold)
)

var (
	CliCmd          = CliCmdC.Sprint
	Success         = SuccessC.Sprint
	Warning         = WarningC.Sprint
	Failure         = FailureC.Sprint
	Troubleshooting = TroubleshootingC.Sprint
	UserInput       = UserInputC.Sprint
	Faint           = FaintC.Sprint
	Bold			= BoldC.Sprint
)
