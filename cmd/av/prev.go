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

var prevFlags struct {
	// should we go to the first
	First bool
}

var prevCmd = &cobra.Command{
	Use:   "prev [<n>|--first]",
	Short: "Checkout the previous branch in the stack",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get the previous branches so we can checkout the nth one
		ctx := cmd.Context()
		repo, err := getRepo(ctx)
		if err != nil {
			return err
		}
		db, err := getDB(ctx, repo)
		if err != nil {
			return err
		}
		tx := db.ReadTx()
		currentBranch, err := repo.CurrentBranchName()
		if err != nil {
			return err
		}
		if repo.IsTrunkBranch(currentBranch) {
			fmt.Fprint(os.Stderr, "already on trunk branch (", colors.UserInput(currentBranch), ")\n")
			return nil
		}
		previousBranches, err := meta.PreviousBranches(tx, currentBranch)
		if err != nil {
			return err
		}

		var branchToCheckout string
		if prevFlags.First {
			if len(previousBranches) == 0 {
				fmt.Fprint(os.Stderr, "already on first branch in stack\n")
				return nil
			}
			branchToCheckout = previousBranches[0]
		} else if len(args) == 0 && len(previousBranches) == 0 {
			branchToCheckout, _ = meta.Trunk(tx, currentBranch)
		} else {
			if len(previousBranches) == 0 {
				return errors.New("there are no previous branches in the stack")
			}
			n := 1
			if len(args) == 1 {
				var err error
				n, err = strconv.Atoi(args[0])
				if err != nil {
					return errors.New("invalid number (unable to parse)")
				}

				if n <= 0 {
					return errors.New("invalid number (must be >= 1)")
				}
			}

			if n > len(previousBranches) {
				return fmt.Errorf("invalid number (there are only %d previous branches in the stack, you can use '--first' to get to first branch in stack)", len(previousBranches))
			}
			branchToCheckout = previousBranches[len(previousBranches)-n]
		}

		if _, err := repo.CheckoutBranch(ctx, &git.CheckoutBranch{
			Name: branchToCheckout,
		}); err != nil {
			return err
		}

		fmt.Fprint(
			os.Stderr,
			"Checked out branch ",
			colors.UserInput(branchToCheckout),
			"\n",
		)

		return nil
	},
}

func init() {
	prevCmd.Flags().BoolVar(
		&prevFlags.First, "first", false,
		"checkout the first branch in the stack",
	)
}
