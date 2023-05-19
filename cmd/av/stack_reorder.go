package main

import (
	"emperror.dev/errors"
	"github.com/spf13/cobra"
	"strings"
)

var stackReorderFlags struct {
	Continue bool
	Abort    bool
}

const stackReorderDoc = `
Interactively reorder the stack.

This is analogous to git rebase --interactive but operates on the stack (rather
than branch) level.

Branches can be re-arranged within the stack and commits can be edited,
squashed, dropped, or moved within the stack.
`

var stackReorderCmd = &cobra.Command{
	Use:   "reorder",
	Short: "reorder the stack",
	Long:  strings.TrimSpace(stackReorderDoc),
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("not implemented")
	},
}

func init() {
	stackReorderCmd.Flags().BoolVar(&stackReorderFlags.Continue, "continue", false, "continue a previous reorder")
	stackReorderCmd.Flags().BoolVar(&stackReorderFlags.Abort, "abort", false, "abort a previous reorder")
}
