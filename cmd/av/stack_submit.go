package main

import (
	"context"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/meta"
	"github.com/spf13/cobra"
)

var stackSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "create pull requests for the current stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			_ = cmd.Usage()
			return errors.New("this command takes no arguments")
		}

		// Get the all branches in the stack
		repo, _, err := getRepoInfo()
		if err != nil {
			return err
		}
		branches, err := meta.ReadAllBranches(repo)
		if err != nil {
			return err
		}

		// ensure pull requests for each branch in the stack
		ctx := context.Background()
		client, err := gh.NewClient(config.Av.GitHub.Token)
		if err != nil {
			return err
		}
		for _, currentMeta := range branches {
			_, err := actions.CreatePullRequest(
				ctx, repo, client,
				actions.CreatePullRequestOpts{
					BranchName: currentMeta.Name,
					Title:      "",
					Body:       "",
					NoPush:     false,
					Force:      false,
					Draft:      false,
				},
			)
			if err != nil {
				return err
			}
		}

		return nil
	},
}
