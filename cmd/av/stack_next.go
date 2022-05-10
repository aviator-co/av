package main

import (
	"emperror.dev/errors"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
)

var stackNextFlags struct {
	// If set, synchronize changes from the parent branch after checking out
	// the next branch.
	Sync bool
}
var stackNextCmd = &cobra.Command{
	Use:   "next <n>",
	Short: "checkout the next branch in the stack",
	Long: strings.TrimSpace(`
Checkout the next branch in the stack.

If the --sync flag is given, this command will also synchronize changes from the
parent branch (i.e., the current branch before this command is run) into the
child branch (without recursively syncing further descendants).
`),
	RunE: func(cmd *cobra.Command, args []string) error {
		var n int = 1
		if len(args) == 1 {
			var err error
			n, err = strconv.Atoi(args[0])
			if err != nil {
				return errors.New("invalid number")
			}
		} else if len(args) > 1 {
			_ = cmd.Usage()
			return errors.New("too many arguments")
		}

		if n <= 0 {
			return errors.New("invalid number (must be >= 1)")
		}

		return errors.New("unimplemented")
	},
}
