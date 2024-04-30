package main

import (
	"context"
	"io"
	"os"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/config"
	"github.com/spf13/cobra"
)

var prCreateFlags struct {
	Draft     bool
	Force     bool
	NoPush    bool
	Title     string
	Body      string
	Edit      bool
	Reviewers []string
}

var prCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "create a pull request for the current branch",
	Long: `Create a pull request for the current branch.

Examples:
  Create a PR with an empty body:
    $ av pr create --title "My PR"

  Create a pull request, specifying the body of the PR from standard input.
    $ av pr create --title "Implement fancy feature" --body - <<EOF
    > Implement my very fancy feature.
    > Can you please review it?
    > EOF

  Create a pull request, assigning reviewers:
    $ av pr create --reviewers "example,@example-org/example-team"
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) (reterr error) {
		repo, err := getRepo()
		if err != nil {
			return err
		}
		branchName, err := repo.CurrentBranchName()
		if err != nil {
			return errors.WrapIf(err, "failed to determine current branch")
		}
		client, err := getGitHubClient()
		if err != nil {
			return err
		}

		db, err := getDB(repo)
		if err != nil {
			return err
		}
		tx := db.WriteTx()
		defer tx.Abort()

		body := prCreateFlags.Body
		// Special case: ready body from stdin
		if prCreateFlags.Body == "-" {
			bodyBytes, err := io.ReadAll(os.Stdin)
			if err != nil {
				return errors.Wrap(err, "failed to read body from stdin")
			}
			prCreateFlags.Body = string(bodyBytes)
		}

		draft := config.Av.PullRequest.Draft
		if cmd.Flags().Changed("draft") {
			draft = prCreateFlags.Draft
		}

		ctx := context.Background()
		res, err := actions.CreatePullRequest(
			ctx, repo, client, tx,
			actions.CreatePullRequestOpts{
				BranchName: branchName,
				Title:      prCreateFlags.Title,
				Body:       body,
				NoPush:     prCreateFlags.NoPush,
				Force:      prCreateFlags.Force,
				Draft:      draft,
				Edit:       prCreateFlags.Edit,
			},
		)
		if err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}

		// Do this after creating the PR and committing the transaction so that
		// our local database is up-to-date even if this fails.
		if len(prCreateFlags.Reviewers) > 0 {
			if err := actions.AddPullRequestReviewers(ctx, client, res.Pull.ID, prCreateFlags.Reviewers); err != nil {
				return err
			}
		}

		if config.Av.PullRequest.WriteStack != "" {
			actions.UpdatePullRequestsWithStackForStack(ctx, client, repo, tx, branchName, config.Av.PullRequest.WriteStack)
		}

		return nil
	},
}

func init() {

	// av pr create
	prCreateCmd.Flags().BoolVar(
		&prCreateFlags.Draft, "draft", false,
		"create the pull request in draft mode",
	)
	prCreateCmd.Flags().BoolVar(
		&prCreateFlags.Force, "force", false,
		"force creation of a pull request even if one already exists",
	)
	prCreateCmd.Flags().BoolVar(
		&prCreateFlags.NoPush, "no-push", false,
		"don't push the latest changes to the remote",
	)
	prCreateCmd.Flags().StringVarP(
		&prCreateFlags.Title, "title", "t", "",
		"title of the pull request to create",
	)
	prCreateCmd.Flags().StringVarP(
		&prCreateFlags.Body, "body", "b", "",
		"body of the pull request to create (a value of - will read from stdin)",
	)
	prCreateCmd.Flags().BoolVar(
		&prCreateFlags.Edit, "edit", false,
		"always open an editor to edit the pull request title and description",
	)
	prCreateCmd.Flags().StringSliceVar(
		&prCreateFlags.Reviewers, "reviewers", nil,
		"add reviewers to the pull request (can be usernames or team names)",
	)
}
