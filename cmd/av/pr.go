package main

import (
	"context"
	"emperror.dev/errors"
	"fmt"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/meta"
	"github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var prCmd = &cobra.Command{
	Use: "pr",
}

var prCreateFlags struct {
	Base  string
	Force bool
}
var prCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "create a pull request for the current branch",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Lets keep this commented until we actually implement this :')
		if config.GitHub.Token == "" {
			// TODO: lets include a documentation link here
			logrus.Error(
				"GitHub token is not configured. " +
					"Please set the github.token field in your config file " +
					"(at ~/.config/av/config.yaml).",
			)
			return errors.New("GitHub token is not configured")
		}

		repo, repoMeta, err := getRepoInfo()
		if err != nil {
			return err
		}

		currentBranch, err := repo.CurrentBranchName()
		if err != nil {
			return errors.WrapIf(err, "failed to determine current branch")
		}
		// figure this out based on whether or not we're on a stacked branch
		var prBaseBranch string

		// TODO:
		//     It would be nice to be able to auto-detect that a PR has been
		//     opened for a given PR without using av. We might need to do this
		//     when creating PRs for a whole stack (e.g., when running `av pr`
		//     on stack branch 3, we should make sure PRs exist for 1 and 2).
		branchMeta, ok := meta.GetBranch(repo, currentBranch)
		if ok && branchMeta.PullRequest.ID != "" && !prCreateFlags.Force {
			return errors.Errorf("This branch already has an associated pull request: %s", branchMeta.PullRequest.Permalink)
		}

		if ok && branchMeta.Parent != "" {
			prBaseBranch = branchMeta.Parent
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

		client, err := gh.NewClient(config.GitHub.Token)
		if err != nil {
			return err
		}
		pull, err := client.CreatePullRequest(context.Background(), githubv4.CreatePullRequestInput{
			RepositoryID: githubv4.ID(repoMeta.ID),
			BaseRefName:  githubv4.String(prBaseBranch),
			HeadRefName:  githubv4.String(currentBranch),
			Title:        githubv4.String(currentBranch),
		})
		if err != nil {
			return err
		}

		branchMeta.PullRequest = meta.PullRequest{
			Number:    pull.Number,
			ID:        pull.ID,
			Permalink: pull.Permalink,
		}
		if err := meta.WriteBranch(repo, branchMeta); err != nil {
			return err
		}

		_, _ = fmt.Printf("Created pull request: %s\n", pull.Permalink)
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
	prCreateCmd.Flags().BoolVar(
		&prCreateFlags.Force, "force", false,
		"force creation of a pull request even if one already exists",
	)
}
