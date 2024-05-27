package main

import (
	"fmt"
	"os"
	"strconv"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/erikgeiser/promptkit/selection"
	"github.com/spf13/cobra"
)

var stackNextFlags struct {
	// should we go to the last
	Last bool
}

var stackNextCmd = &cobra.Command{
	Use:     "next [<n>|--last]",
	Aliases: []string{"n"},
	Short:   "checkout the next branch in the stack",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		if len(meta.Children(tx, currentBranch)) == 0 {
			return errors.New("there is no next branch")
		}

		if stackNextFlags.Last {
			for len(meta.Children(tx, currentBranch)) > 0 {
				selectedChild, err := nextChild(tx, currentBranch)
				if err != nil {
					return err
				}

				currentBranch = selectedChild.Name
			}
			return checkoutBranch(currentBranch, repo)
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

		for i := 0; i < n; i++ {
			if len(meta.Children(tx, currentBranch)) == 0 {
				return fmt.Errorf("invalid number (there are only %d subsequent branches in the stack)", i)
			}

			selectedChild, err := nextChild(tx, currentBranch)
			if err != nil {
				return err
			}

			currentBranch = selectedChild.Name
		}

		return checkoutBranch(currentBranch, repo)

	},
}

func init() {
	stackNextCmd.Flags().BoolVar(
		&stackNextFlags.Last, "last", false,
		"go to the last branch in the current stack",
	)
}

func checkoutBranch(branchname string, repo *git.Repo) error {
	if _, err := repo.CheckoutBranch(&git.CheckoutBranch{
		Name: branchname,
	}); err != nil {
		return err
	}

	fmt.Fprint(
		os.Stderr,
		"Checked out branch ",
		colors.UserInput(branchname),
		"\n",
	)

	return nil
}

// nextChild prompts the user to select the next child branch to follow or returns the only child if there is only one.
func nextChild(tx meta.ReadTx, branchName string) (meta.Branch, error) {
	children := meta.Children(tx, branchName)
	if len(children) == 0 {
		return meta.Branch{}, errors.New("no children")
	}

	if len(children) == 1 {
		return children[0], nil
	}

	options := make([]string, 0, len(children))
	for _, child := range children {
		options = append(options, child.Name)
	}

	sp := selection.New(fmt.Sprintf("There are multiple children of branch %s. Which branch would you like to follow?", colors.UserInput(branchName)), options)
	sp.PageSize = 4
	sp.Filter = nil

	choice, err := sp.RunPrompt()
	if err != nil {
		return meta.Branch{}, err
	}

	for _, child := range children {
		if child.Name == choice {
			return child, nil
		}
	}

	return meta.Branch{}, errors.New("could not find next branch")
}
