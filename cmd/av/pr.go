package main

import (
	"emperror.dev/errors"
	"fmt"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/stacks"
	"github.com/spf13/cobra"
)

var prCmd = &cobra.Command{
	Use: "pr",
}

var prCreateFlags struct {
	Base string
}
var prCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "create a pull request for the current branch",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Lets keep this commented until we actually implement this :')
		//if config.GitHub.Token == "" {
		//	// TODO: lets include a documentation link here
		//	logrus.Info(
		//		"GitHub token is not configured. " +
		//			"Consider adding it to your config file (at ~/.config/av/config.yaml) " +
		//			"to allow av to automatically create pull requests.",
		//	)
		//}

		repo, err := getRepo()
		if err != nil {
			return errors.WrapIf(err, "failed to get repo")
		}
		currentBranch, err := repo.CurrentBranchName()
		if err != nil {
			return errors.WrapIf(err, "failed to determine current branch")
		}
		origin, err := repo.Origin()
		if err != nil {
			return errors.WrapIf(err, "failed to determine origin")
		}

		// figure this out based on whether or not we're on a stacked branch
		var prBaseBranch string

		stackMetadata := stacks.GetMetadata(repo, currentBranch)
		if stackMetadata != nil {
			prBaseBranch = stackMetadata.Parent
		} else {
			defaultBranch, err := repo.DefaultBranch()
			if err != nil {
				return errors.WrapIf(err, "failed to determine default branch")
			}
			if currentBranch == defaultBranch {
				return errors.Errorf(
					"cannot create pull request for default branch %q "+
						"(did you mean to checkout a different branch before running this command?)",
					defaultBranch,
				)
			}
			prBaseBranch = defaultBranch
		}

		// Example:
		// https://github.com/aviator-co/av/compare/master...my-fancy-feature?quick_pull=1
		_, _ = fmt.Printf(
			"%s/%s/compare/%s...%s?quick_pull=1\n",
			config.GitHub.BaseUrl,
			origin.RepoSlug,
			prBaseBranch,
			currentBranch,
		)
		return nil
	},
}

func init() {
	prCmd.AddCommand(prCreateCmd)

	// av pr create
	prCreateCmd.Flags().StringVar(
		&prCreateFlags.Base, "base", "",
		"base branch to create the pull request against",
	)
}
