package main

import (
	"github.com/spf13/cobra"
)

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "manage pull requests",
}

func init() {
	prCmd.AddCommand(
		prCreateCmd,
		prStatusCmd,
	)
}
