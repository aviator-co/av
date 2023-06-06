package main

import (
	"context"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/cleanup"
	"github.com/shurcooL/githubv4"
	"github.com/spf13/cobra"
)

var stackSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Create pull requests for every branch in the stack",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		// Get the all branches in the stack
		repo, err := getRepo()
		if err != nil {
			return err
		}

		db, err := getDB(repo)
		if err != nil {
			return err
		}
		tx := db.WriteTx()
		cu := cleanup.New(func() { tx.Abort() })
		defer cu.Cleanup()

		currentBranch, err := repo.CurrentBranchName()
		if err != nil {
			return err
		}
		previousBranches, err := meta.PreviousBranches(tx, currentBranch)
		if err != nil {
			return err
		}
		subsequentBranches := meta.SubsequentBranches(tx, currentBranch)
		var branchesToSubmit []string
		branchesToSubmit = append(branchesToSubmit, previousBranches...)
		branchesToSubmit = append(branchesToSubmit, currentBranch)
		branchesToSubmit = append(branchesToSubmit, subsequentBranches...)

		// ensure pull requests for each branch in the stack
		ctx := context.Background()
		client, err := getClient(config.Av.GitHub.Token)
		if err != nil {
			return err
		}
		for _, branchName := range branchesToSubmit {
			// TODO: should probably commit database after every call to this
			// since we're just syncing state from GitHub
			result, err := actions.CreatePullRequest(
				ctx, repo, client, tx,
				actions.CreatePullRequestOpts{
					BranchName: branchName,
					Draft:      config.Av.PullRequest.Draft,
				},
			)
			if err != nil {
				return err
			}
			// make sure the base branch of the PR is up to date if it already exists
			if !result.Created && result.Pull.BaseRefName != result.Branch.Parent.Name {
				if _, err := client.UpdatePullRequest(
					ctx, githubv4.UpdatePullRequestInput{
						PullRequestID: githubv4.ID(result.Branch.PullRequest.ID),
						BaseRefName:   gh.Ptr(githubv4.String(result.Branch.Parent.Name)),
					},
				); err != nil {
					return errors.Wrap(err, "failed to update PR base branch")
				}
			}
		}

		cu.Cancel()
		if err := tx.Commit(); err != nil {
			return err
		}
		return nil
	},
}
