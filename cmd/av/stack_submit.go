package main

import (
	"context"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/meta"
	"github.com/shurcooL/githubv4"
	"github.com/spf13/cobra"
)

var stackSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "create/synchronize pull requests for the current stack",
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
		currentBranch, err := repo.CurrentBranchName()
		if err != nil {
			return err
		}
		previousBranches, err := meta.PreviousBranches(branches, currentBranch)
		if err != nil {
			return err
		}
		subsequentBranches, err := meta.SubsequentBranches(branches, currentBranch)
		if err != nil {
			return err
		}
		var branchesToSubmit []string
		branchesToSubmit = append(branchesToSubmit, previousBranches...)
		branchesToSubmit = append(branchesToSubmit, currentBranch)
		branchesToSubmit = append(branchesToSubmit, subsequentBranches...)

		// ensure pull requests for each branch in the stack
		ctx := context.Background()
		client, err := gh.NewClient(config.Av.GitHub.Token)
		if err != nil {
			return err
		}
		for _, branchName := range branchesToSubmit {
			result, err := actions.CreatePullRequest(
				ctx, repo, client,
				actions.CreatePullRequestOpts{
					BranchName: branchName,
					Draft:      config.Av.PullRequest.Draft,
				},
			)
			if err != nil {
				return err
			}
			// make sure the base branch of the PR is up to date if it already exists
			if !result.Created {
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

		return nil
	},
}
