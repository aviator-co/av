package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var authStatusCmd = &cobra.Command{
	Use:    "status",
	Short:  "Deprecated: Show info about the logged in user (use 'av auth' instead)",
	Hidden: true,
	Args:   cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("'av auth status' is deprecated. Please use 'av auth' instead.")
		authCmd.Run(cmd, args)
	},
}
