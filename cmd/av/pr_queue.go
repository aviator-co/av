package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/avgql"
	"github.com/shurcooL/githubv4"
	"github.com/shurcooL/graphql"
	"github.com/spf13/cobra"
)

var prQueueFlags struct {
	SkipLine 	bool
	Targets		string
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
						Title        graphql.String
						Status       graphql.String
						StatusReason graphql.String
						Author       struct {
							Login graphql.String
						}
						CreatedAt             githubv4.DateTime
						QueuedAt              githubv4.DateTime
						MergedAt              githubv4.DateTime
						BaseBranchName        graphql.String
						HeadBranchName        graphql.String
						RequiredCheckStatuses []struct {
							RequiredCheck struct {
								Pattern graphql.String
							}
							Result graphql.String
						}
					}
				} `graphql:"... on QueuePullRequestPayload"`
					
			} `graphql:"queuePullRequest(input: {repoOwner: $repoOwner, repoName:$repoName, number:$prNumber})"`
		}
		err = client.Mutate(context.Background(), &mutation, variables)
		if err != nil {
			return fmt.Errorf("Attempt to mutate failed; %#+v", err)
		}
		// We print to stderr everywhere, why?
		// Also, should we surface anything else to the user? bar improving what we actually 
		pr := mutation.QueuePullRequest.QueuePullRequestPayload.PullRequest

		fmt.Fprintf(
			os.Stderr, 
			"Pull request was added to the mergequeue.\n", 
			pr.BaseBranchName,
			"\n",
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
}
