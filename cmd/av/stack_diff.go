package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var stackDiffCmd = &cobra.Command{
	Use:          "diff",
	Short:        "generate diff between working tree and the parent branch",
	Long: strings.TrimSpace(`
Generates the diff between the working tree and the parent branch 
(i.e., the diff between the current branch and the previous branch in the stack).
`),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		currentBranchName, err := repo.CurrentBranchName()
		if err != nil {
			return err
		}

		branch, _ := meta.ReadBranch(repo, currentBranchName)

		diff, err := repo.Diff(&git.DiffOpts{
			Color: !color.NoColor,
			Commit: branch.Parent.Name,
		})
		if err != nil {
			return err
		}

		_, _ = fmt.Fprint(os.Stderr, diff.Contents)

		return nil
	},
}
