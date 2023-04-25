package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/spf13/cobra"
)

var stackDiffCmd = &cobra.Command{
	Use:          "diff",
	Short:        "generate diff between working tree and current index",
	Long: strings.TrimSpace(`
Generates the diff between the working tree and the current index (i.e., the diff containing all unstaged changes).
`),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		diff, err := repo.Diff(&git.DiffOpts{})
		if err != nil {
			return err
		}

		diffLines := strings.Split(diff.Contents, "\n")

		for _, line := range diffLines {
			if strings.HasPrefix(line, "diff ") ||
				strings.HasPrefix(line, "index ") ||
				strings.HasPrefix(line, "+++") ||
				strings.HasPrefix(line, "---") {
				_, _ = fmt.Fprint(os.Stderr,
					colors.Bold(line),
					"\n",
				)
			} else if strings.HasPrefix(line, "+") {
				_, _ = fmt.Fprint(os.Stderr,
					colors.Success(line),
					"\n",
				)
			} else if strings.HasPrefix(line, "-") {
				_, _ = fmt.Fprint(os.Stderr,
					colors.Failure(line),
					"\n",
				)
			} else {
				_, _ = fmt.Fprint(os.Stderr,
					colors.Faint(line),
					"\n",
				)
			}
		}

		return nil
	},
}
