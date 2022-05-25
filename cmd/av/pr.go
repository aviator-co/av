package main

import (
	"context"
	"emperror.dev/errors"
	"fmt"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"strings"
)

var prCmd = &cobra.Command{
	Use: "pr",
}

var prCreateFlags struct {
	Base   string
	Force  bool
	NoPush bool
	Title  string
	Body   string
}
var prCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "create a pull request for the current branch",
	Long: strings.TrimSpace(`
Create a pull request for the current branch.

Examples:
  Create a PR with an empty body:
    $ av pr create --title "My PR"

  Create a pull request, specifying the body of the PR from standard input.
    $ av pr create --title "Implement fancy feature" --body - <<EOF
    > Implement my very fancy feature.
    > Can you please review it?
    > EOF
`),
	RunE: func(cmd *cobra.Command, args []string) error {
		// arg validation
		if prCreateFlags.Title == "" {
			return errors.New("title is required")
		}

		repo, repoMeta, err := getRepoInfo()
		if err != nil {
			return err
		}

		currentBranch, err := repo.CurrentBranchName()
		if err != nil {
			return errors.WrapIf(err, "failed to determine current branch")
		}

		if !prCreateFlags.NoPush {
			pushFlags := []string{"push"}

			// Check if the upstream is set. If not, we set it during push.
			upstream, err := repo.RevParse(&git.RevParse{
				SymbolicFullName: true,
				Rev:              "HEAD@{u}",
			})
			if err != nil {
				// Set the upstream branch
				upstream = "refs/remotes/origin/" + currentBranch
				pushFlags = append(pushFlags, "--set-upstream", "origin", currentBranch)
			}
			logrus.WithField("upstream", upstream).Debug("pushing latest changes")
			if _, err := repo.Git(pushFlags...); err != nil {
				return errors.WrapIf(err, "failed to push")
			}
		}

		// TODO:
		//     It would be nice to be able to auto-detect that a PR has been
		//     opened for a given PR without using av. We might need to do this
		//     when creating PRs for a whole stack (e.g., when running `av pr`
		//     on stack branch 3, we should make sure PRs exist for 1 and 2).
		branchMeta, ok := meta.ReadBranch(repo, currentBranch)
		if ok && branchMeta.PullRequest != nil && !prCreateFlags.Force {
			return errors.Errorf("This branch already has an associated pull request: %s", branchMeta.PullRequest.Permalink)
		}

		// figure this out based on whether or not we're on a stacked branch
		var prBaseBranch string
		if ok && branchMeta.Parent != "" {
			prBaseBranch = branchMeta.Parent
			// check if the base branch also has an associated PR
			baseMeta, ok := meta.ReadBranch(repo, prBaseBranch)
			if !ok {
				return errors.WrapIff(err, "failed to read branch metadata for %q", prBaseBranch)
			}
			if baseMeta.PullRequest == nil {
				// TODO:
				//     We should automagically create PRs for every branch in the stack
				return errors.Errorf(
					"base branch %q does not have an associated pull request "+
						"(create one by checking out the branch and running `av pr create`)",
					prBaseBranch,
				)
			}
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

		body := prCreateFlags.Body
		// Special case: ready body from stdin
		if body == "-" {
			bodyBytes, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				return errors.Wrap(err, "failed to read body from stdin")
			}
			body = string(bodyBytes)
		}

		pull, err := client.CreatePullRequest(context.Background(), githubv4.CreatePullRequestInput{
			RepositoryID: githubv4.ID(repoMeta.ID),
			BaseRefName:  githubv4.String(prBaseBranch),
			HeadRefName:  githubv4.String(currentBranch),
			Title:        githubv4.String(prCreateFlags.Title),
			Body:         gh.Ptr(githubv4.String(body)),
		})
		if err != nil {
			return err
		}

		branchMeta.PullRequest = &meta.PullRequest{
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
	prCreateCmd.Flags().BoolVar(
		&prCreateFlags.NoPush, "no-push", false,
		"don't push the latest changes to the remote",
	)
	// TODO:
	//     Want to automatically determine the title of the PR, probably using
	//     the headline of the first commit.
	prCreateCmd.Flags().StringVarP(
		&prCreateFlags.Title, "title", "t", "",
		"title of the pull request to create",
	)
	prCreateCmd.Flags().StringVarP(
		&prCreateFlags.Body, "body", "b", "",
		"body of the pull request to create (a value of - will read from stdin)",
	)
	_ = prCreateCmd.MarkFlagRequired("title")
}
