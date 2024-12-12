package main

import (
	"fmt"
	"strings"

	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var tidyCmd = &cobra.Command{
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

		deleted, orphaned, err := actions.TidyDB(repo, db)
		if err != nil {
			return err
		}

		var ss []string

		hasDeleted := false
		for _, d := range deleted {
			if d {
				hasDeleted = true
				break
			}
		}

		if hasDeleted {
			ss = append(
				ss,
				colors.SuccessStyle.Render(
					"✓ Updated the branch metadata for the deleted branches",
				),
			)
			ss = append(ss, "")
			ss = append(ss, "  Following branches no longer exist in the repository:")
			ss = append(ss, "")
			for name, d := range deleted {
				if d {
					ss = append(ss, "  * "+name)
				}
			}
			if len(orphaned) > 0 {
				ss = append(ss, "")
				ss = append(
					ss,
					"  Following branches are orphaned since they have deleted parents:",
				)
				ss = append(ss, "")
				for name := range orphaned {
					ss = append(ss, "  * "+name)
				}
				ss = append(ss, "")
				ss = append(ss, "  The orphaned branches still exist in the repository.")
				ss = append(ss, "  You can re-adopt them to av by running 'av adopt'.")
			}
		} else {
			ss = append(ss, colors.SuccessStyle.Render("✓ No branch to tidy"))
		}

		var ret string
		if len(ss) != 0 {
			ret = lipgloss.NewStyle().MarginTop(1).MarginBottom(1).MarginLeft(2).Render(
				lipgloss.JoinVertical(0, ss...),
			) + "\n"
		}
		fmt.Print(ret)

		return nil
	},
}
