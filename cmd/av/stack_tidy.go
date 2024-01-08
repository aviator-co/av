package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/textutils"
	"github.com/spf13/cobra"
)

var stackTidyCmd = &cobra.Command{
	Use:   "tidy",
	Short: "Tidy stacked branches",
	Long: strings.TrimSpace(`
Tidy stacked branches by removing deleted or merged branches.

This command detects which branches are deleted or merged and re-parents
children of merged branches. This operates on only av's internal metadata and
does not delete Git branches.
`),
	SilenceUsage: true,
	Args:         cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		db, err := getDB(repo)
		if err != nil {
			return err
		}

		nDeleted, err := actions.TidyDB(repo, db)
		if err != nil {
			return err
		}

		if nDeleted > 0 {
			_, _ = fmt.Fprint(os.Stderr,
				"Tidied ", colors.UserInput(nDeleted), " ",
				textutils.Pluralize(nDeleted, "branch", "branches"),
				".\n",
			)
		} else {
			_, _ = fmt.Fprintln(os.Stderr, "No branches to tidy.")
		}
		return nil
	},
}
