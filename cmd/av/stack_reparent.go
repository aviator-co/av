package main

import (
	"fmt"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/spf13/cobra"
	"os"
)

var stackReparentCmd = &cobra.Command{
	Use:    "reparent <new-parent>",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, _ = fmt.Fprint(os.Stderr,
			colors.Failure("ERROR: "),
			"The ", colors.CliCmd("av stack reparent"), " command is deprecated: ",
			"use ", colors.CliCmd("av stack sync --parent <new-parent>"), " instead.",
			"\n",
		)
		os.Exit(1)
		return nil
	},
}
