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

var stackPrevCmd = &cobra.Command{
	Use:   "prev <n>",
	Short: "checkout the previous branch in the stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		var n int = 1
		if len(args) == 1 {
			var err error
			n, err = strconv.Atoi(args[0])
			if err != nil {
				return errors.Wrap(err, "invalid number")
			}
		} else if len(args) > 1 {
			_ = cmd.Usage()
			return errors.New("too many arguments")
		}

		if n <= 0 {
			return errors.New("invalid number (must be >= 1)")
		}

		// Get the previous branches so we can checkout the nth one
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
		previousBranches, err := previousBranches(branches, currentBranch)
		if err != nil {
			return err
		}

		// confirm we can in fact do the operation given current branch state
		if len(previousBranches) == 0 {
			return errors.New("there is no previous branch")
		}
		if n > len(previousBranches) {
			return fmt.Errorf("invalid number (there are only %d previous branches in the stack)", len(previousBranches))
		}

		// checkout nth previous branch
		var branchToCheckout = previousBranches[len(previousBranches)-n]
		if _, err := repo.CheckoutBranch(&git.CheckoutBranch{
			Name: branchToCheckout,
		}); err != nil {
			return err
		}

		_, _ = fmt.Fprint(os.Stderr, "Checked out branch ", colors.UserInput(branchToCheckout))

		return nil
	},
}
