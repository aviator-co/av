package main

import (
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
}

func init() {
	authCmd.AddCommand(
		authStatusCmd,
	)
}
