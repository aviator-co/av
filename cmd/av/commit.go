package main

import (
	"github.com/spf13/cobra"
)

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "operate on commits",
}

func init() {
	commitCmd.AddCommand(
		commitSplitCmd, 
		commitCreateCmd,
		commitAmendCmd,
	)
}
