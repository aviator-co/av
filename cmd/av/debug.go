package main

import (
	"context"
	"fmt"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/gh"
	"github.com/shurcooL/githubv4"
	"github.com/spf13/cobra"
)

var debugCmd = &cobra.Command{
	Use: "debug",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := gh.NewClient(config.GitHub.Token)
		if err != nil {
			return err
		}
		ctx := context.Background()
		prs, err := client.RepoPullRequests(ctx, gh.RepoPullRequestOpts{
			Owner:  "travigd",
			Repo:   "mergequeue-demo",
			States: []githubv4.PullRequestState{githubv4.PullRequestStateOpen},
		})
		if err != nil {
			return err
		}
		for _, pr := range prs.PullRequests {
			fmt.Printf("%s (%s => %s)\n", pr.Title, pr.HeadBranchName(), pr.BaseBranchName())
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(debugCmd)
}
