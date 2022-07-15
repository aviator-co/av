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

var stackNextCmd = &cobra.Command{
	Use:   "next <n>",
	Short: "checkout the next branch in the stack",
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

		// Get the subsequent branches so we can checkout the nth one
		repo, _, err := getRepoInfo()
		if err != nil {
			return err
		}
		branches, err := meta.ReadAllBranches(repo)
		if err != nil {
			return err
		}
		currentBranch, err := repo.CurrentBranchName()
		if err != nil {
			return err
		}
		subsequentBranches, err := subsequentBranches(branches, currentBranch)
		if err != nil {
			return err
		}

		// confirm we can in fact do the operation given current branch state
		if len(subsequentBranches) == 0 {
			return errors.New("there is no next branch")
		}
		if n > len(subsequentBranches) {
			return fmt.Errorf("invalid number (there are only %d subsequent branches in the stack)", len(subsequentBranches))
		}

		// checkout nth branch
		var branchToCheckout = subsequentBranches[n-1]
		if _, err := repo.CheckoutBranch(&git.CheckoutBranch{
			Name: branchToCheckout,
		}); err != nil {
			return err
		}

		_, _ = fmt.Fprint(os.Stderr, "Checked out branch ", colors.UserInput(branchToCheckout))

		return nil
	},
}
