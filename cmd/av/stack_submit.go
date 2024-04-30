package main

import (
	"context"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/cleanup"
	"github.com/shurcooL/githubv4"
	"github.com/spf13/cobra"
)

var stackSubmitFlags struct {
	Current bool
}

var stackSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Create pull requests for every branch in the stack",
	Long: strings.TrimSpace(`
	Create pull requests for every branch in the stack

If the --current flag is given, this command will create pull requests up to the current branch.`),
	Args: cobra.NoArgs,
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

		currentStackBranches, err := meta.StackBranches(tx, currentBranch)

		var branchesToSubmit []string
		if stackSubmitFlags.Current {
			previousBranches, err := meta.PreviousBranches(tx, currentBranch)
			if err != nil {
				return err
			}
			branchesToSubmit = append(branchesToSubmit, previousBranches...)
			branchesToSubmit = append(branchesToSubmit, currentBranch)
		} else {
			branchesToSubmit = currentStackBranches
		}

		// ensure pull requests for each branch in the stack
		var lastCreatedPullRequest *meta.PullRequest
		ctx := context.Background()
		client, err := getGitHubClient()
		if err != nil {
			return err
		}
		for _, branchName := range branchesToSubmit {
			// TODO: should probably commit database after every call to this
			// since we're just syncing state from GitHub
			result, err := actions.CreatePullRequest(
				ctx, repo, client, tx,
				actions.CreatePullRequestOpts{
					BranchName:    branchName,
					Draft:         config.Av.PullRequest.Draft,
					NoOpenBrowser: true,
				},
			)
			if err != nil {
				return err
			}
			if result.Created {
				lastCreatedPullRequest = result.Branch.PullRequest
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

		if config.Av.PullRequest.WriteStack != "" {
			if err = actions.UpdatePullRequestsWithStack(ctx, client, repo, tx, currentStackBranches, config.Av.PullRequest.WriteStack); err != nil {
				return err
			}
		}

		if lastCreatedPullRequest != nil && config.Av.PullRequest.OpenBrowser {
			actions.OpenPullRequestInBrowser(lastCreatedPullRequest.Permalink)
		}

		return nil
	},
}

func init() {
	stackSubmitCmd.Flags().BoolVar(
		&stackSubmitFlags.Current, "current", false,
		"only create pull requests up to the current branch",
	)
}
