package main

import (
	"context"
	"emperror.dev/errors"
	"encoding/json"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/gh"
	"github.com/spf13/cobra"
	"os"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "fetch latest information from GitHub",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := gh.NewClient(config.GitHub.Token)
		if err != nil {
			return errors.Wrap(err, "failed to create GitHub client")
		}
		pull, err := client.PullRequest(context.Background(), gh.PullRequestOpts{
			Owner:  "travigd",
			Repo:   "private-playground",
			Number: 1,
		})
		if err != nil {
			return errors.WrapIf(err, "failed to fetch pull request")
		}
		data, err := json.MarshalIndent(pull, "", "  ")
		if err != nil {
			return errors.WrapIf(err, "failed to marshal pull request")
		}
		_, err = os.Stdout.Write(data)
		if err != nil {
			return errors.WrapIf(err, "failed to write pull request")
		}

		return errors.New("unimplemented")
	},
}
