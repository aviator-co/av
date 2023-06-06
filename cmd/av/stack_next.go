package main

import (
	"fmt"
	"os"
	"strconv"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/spf13/cobra"
)

var stackNextFlags struct {
	// should we go to the last
	Last bool
}

var stackNextCmd = &cobra.Command{
	Use:   "next [<n>|--last]",
	Short: "checkout the next branch in the stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get the subsequent branches so we can checkout the nth one
		repo, err := getRepo()
		if err != nil {
			return err
		}

		db, err := getDB(repo)
		if err != nil {
			return err
		}
		tx := db.ReadTx()

		currentBranch, err := repo.CurrentBranchName()
		if err != nil {
			return err
		}
		subsequentBranches := meta.SubsequentBranches(tx, currentBranch)

		var branchToCheckout string
		if stackNextFlags.Last {
			if len(subsequentBranches) == 0 {
				return errors.New("already on last branch in stack\n")
			}
			branchToCheckout = subsequentBranches[len(subsequentBranches)-1]
		} else {
			if len(subsequentBranches) == 0 {
				return errors.New("there is no next branch")
			}
			var n int = 1
			if len(args) == 1 {
				var err error
				n, err = strconv.Atoi(args[0])
				if err != nil {
					return errors.New("invalid number (unable to parse)")
				}
			} else if len(args) > 1 {
				_ = cmd.Usage()
				return errors.New("too many arguments")
			}
			if n <= 0 {
				return errors.New("invalid number (must be >= 1)")
			}
			if n > len(subsequentBranches) {
				return fmt.Errorf("invalid number (there are only %d subsequent branches in the stack)", len(subsequentBranches))
			}
			branchToCheckout = subsequentBranches[n-1]
		}

		if _, err := repo.CheckoutBranch(&git.CheckoutBranch{
			Name: branchToCheckout,
		}); err != nil {
			return err
		}

		_, _ = fmt.Fprint(
			os.Stderr,
			"Checked out branch ",
			colors.UserInput(branchToCheckout),
			"\n",
		)

		return nil
	},
}

func init() {
	stackNextCmd.Flags().BoolVar(
		&stackNextFlags.Last, "last", false,
		"go to the last branch in the current stack",
	)
}
