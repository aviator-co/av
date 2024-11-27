package main

import (
	"strings"

	"github.com/spf13/cobra"
)

var stackSubmitFlags struct {
	Current bool
	Draft   bool
}

var stackSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Create pull requests for every branch in the stack",
	Long: strings.TrimSpace(`
Create pull requests for every branch in the stack

If the --current flag is given, this command will create pull requests up to the current branch.`),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return submitAll(stackSubmitFlags.Current, stackSubmitFlags.Draft)
	},
}
