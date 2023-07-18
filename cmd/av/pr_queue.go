package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/sirupsen/logrus"

	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/avgql"
	"github.com/shurcooL/graphql"
	"github.com/spf13/cobra"
)

var prQueueFlags struct {
	SkipLine bool
	Targets  string
}

var prQueueCmd = &cobra.Command{
	Use:          "queue",
	Short:        "queue a pull request for the current branch",
	SilenceUsage: true,
	Args:         cobra.NoArgs,
	// error or reterr from emperror.dev/errors here?
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		db, err := getDB(repo)
		if err != nil {
			return err
		}

		tx := db.ReadTx()
		currentBranchName, err := repo.CurrentBranchName()
		if err != nil {
			return err
		}

		branch, _ := tx.Branch(currentBranchName)
		if branch.PullRequest == nil {
			return errors.New(
				"this branch has no associated pull request (run `av pr create` to create one)",
			)
		}

		prNumber := branch.PullRequest.Number
		repository, exists := tx.Repository()
		if !exists {
			return actions.ErrRepoNotInitialized
		}

		var variables = map[string]interface{}{
			"repoOwner": graphql.String(repository.Owner),
			"repoName":  graphql.String(repository.Name),
			"prNumber":  graphql.Int(prNumber),
		}

		// I have a feeling this would be better written inside of av/internals
		client := avgql.NewClient()

		var mutation struct {
			QueuePullRequest struct {
				QueuePullRequestPayload struct {
					PullRequest struct {
						// We don't currently use anything here, but we need to select
						// at least one field to make the GraphQL query valid.
						Status graphql.String
					}
				} `graphql:"... on QueuePullRequestPayload"`
			} `graphql:"queuePullRequest(input: {repoOwner: $repoOwner, repoName:$repoName, number:$prNumber})"`
		}

		err = client.Mutate(context.Background(), &mutation, variables)
		if err != nil {
			logrus.WithError(err).Debug("failed to queue pull request")
			return fmt.Errorf("failed to queue pull request: %s", err)
		}
		_, _ = fmt.Fprint(
			os.Stderr,
			"Queued pull request ", colors.UserInput(branch.PullRequest.Permalink), ".\n",
		)

		return nil
	},
}

func init() {
	prQueueCmd.Flags().BoolVar(
		&prQueueFlags.SkipLine, "skip-line", false,
		"skip in front of the existing pull requests, merge this pull request right now",
	)
	prQueueCmd.Flags().StringVarP(
		&prQueueFlags.Targets, "targets", "t", "",
		"additional targets affected by this pull request",
	)
	// These flags are not yet supported. 
	prQueueCmd.Flags().MarkHidden("targets")
	prQueueCmd.Flags().MarkHidden("skip-line")

}

