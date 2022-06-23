package main

import (
	"context"
	"fmt"
	"os"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/fatih/color"
	"github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "fetch latest state from GitHub",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, info, err := getRepoInfo()
		if err != nil {
			return err
		}
		branches, err := meta.ReadAllBranches(repo)
		if err != nil {
			return errors.Wrap(err, "failed to read av branch metadata")
		}

		client, err := gh.NewClient(config.Av.GitHub.Token)
		if err != nil {
			return err
		}

		ctx := context.Background()
		var cursor string
		updatedCount := 0
		for {
			prsPage, err := client.RepoPullRequests(ctx, gh.RepoPullRequestOpts{
				Owner:  info.Owner,
				Repo:   info.Name,
				After:  cursor,
				States: []githubv4.PullRequestState{githubv4.PullRequestStateOpen},
			})
			if err != nil {
				return errors.Wrap(err, "failed to fetch pull requests from GitHub")
			}
			if cursor == "" {
				// only do this once at the start
				_, _ = fmt.Fprint(
					os.Stderr,
					"Fetching ", colors.UserInput(prsPage.TotalCount),
					" open pull requests from GitHub...",
					"\n",
				)
			}

			for _, pr := range prsPage.PullRequests {
				// TODO: maybe warn if local branch is not up-to-date with remote?
				branchMeta, ok := branches[pr.HeadBranchName()]
				if !ok {
					logrus.WithField("branch", pr.HeadBranchName()).Debug("skipping PR for unknown local branch")
					continue
				}
				logrus.WithField("branch", pr.HeadBranchName()).Debug("found PR for known local branch")
				if branchMeta.PullRequest == nil {
					_, _ = fmt.Fprint(
						os.Stderr,
						"  - Found pull request ", colors.UserInput(pr.Number),
						" for branch ", colors.UserInput(pr.HeadBranchName()),
						"\n",
					)
				} else if branchMeta.PullRequest.Number != pr.Number {
					// This shouldn't usually ever happen, not sure what the
					// best thing to do here, but this handling allows you to
					// close a PR then open a new one and then run `av fetch`
					_, _ = fmt.Fprint(
						os.Stderr,
						"  - ", color.RedString("WARNING: "),
						"found new pull request ", colors.UserInput("#", pr.Number, " ", pr.Title),
						" for branch ", colors.UserInput(pr.HeadBranchName()),
						", overwriting... ",
						" (old pull request: ", colors.UserInput("#", branchMeta.PullRequest.Number), ")",
						"\n",
					)
				} else {
					// nothing to do, we already have the PR stored in metadata
					continue
				}
				updatedCount++
				branchMeta.PullRequest = &meta.PullRequest{
					ID:        pr.ID,
					Number:    pr.Number,
					Permalink: pr.Permalink,
				}
				if err := meta.WriteBranch(repo, branchMeta); err != nil {
					return errors.Wrap(err, "failed to write branch metadata")
				}
			}

			if prsPage.HasNextPage {
				cursor = prsPage.EndCursor
			} else {
				break
			}
		}

		_, _ = fmt.Fprint(
			os.Stderr,
			"Updated ", color.GreenString("%d", updatedCount), " pull requests",
			"\n",
		)
		return nil
	},
}
