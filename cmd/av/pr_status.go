package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/avgql"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/timeutils"
	"github.com/shurcooL/githubv4"
	"github.com/shurcooL/graphql"
	"github.com/spf13/cobra"
)

var prStatusCmd = &cobra.Command{
	Use:          "status",
	Short:        "check pr status",
	SilenceUsage: true,
	Args:         cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		variables, err := getQueryVariables()
		if err != nil {
			return err
		}

		client := avgql.NewClient()

		var query struct {
			GithubRepository struct {
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
					// TODO: add BotPR info
				} `graphql:"pullRequest(number: $prNumber)"`
			} `graphql:"githubRepository(owner: $repoOwner, name:$repoName)"`
		}

		err = client.Query(context.Background(), &query, variables)
		if err != nil {
			return err
		}

		indent := "    "
		prStatus := query.GithubRepository.PullRequest.Status

		// Print PR info
		_, _ = fmt.Fprint(
			os.Stderr,
			"#",
			variables["prNumber"],
			" ",
			colors.UserInput(query.GithubRepository.PullRequest.Title),
			"\n",
		)
		_, _ = fmt.Fprint(os.Stderr, indent, "Status: ", colors.UserInput(prStatus))

		if prStatus == "PENDING" || prStatus == "BLOCKED" {
			_, _ = fmt.Fprint(
				os.Stderr,
				indent,
				" (",
				colors.UserInput(query.GithubRepository.PullRequest.StatusReason),
				")",
			)
		}
		_, _ = fmt.Fprint(os.Stderr, "\n")

		_, _ = fmt.Fprint(
			os.Stderr,
			indent,
			"Author: ",
			colors.UserInput(query.GithubRepository.PullRequest.Author.Login),
			"\n",
		)
		_, _ = fmt.Fprint(
			os.Stderr,
			indent,
			"Created at: ",
			colors.UserInput(
				timeutils.FormatLocal(query.GithubRepository.PullRequest.CreatedAt.Time),
			),
			"\n",
		)

		if prStatus == "QUEUED" {
			_, _ = fmt.Fprint(
				os.Stderr,
				indent,
				"Queued at: ",
				colors.UserInput(
					timeutils.FormatLocal(query.GithubRepository.PullRequest.QueuedAt.Time),
				),
				"\n",
			)
		}
		if prStatus == "MERGED" {
			_, _ = fmt.Fprint(
				os.Stderr,
				indent,
				"Merged at: ",
				colors.UserInput(
					timeutils.FormatLocal(query.GithubRepository.PullRequest.MergedAt.Time),
				),
				"\n",
			)
		}

		_, _ = fmt.Fprint(
			os.Stderr,
			indent,
			"Base branch: ",
			colors.UserInput(
				query.GithubRepository.PullRequest.BaseBranchName,
				" <- ",
				query.GithubRepository.PullRequest.HeadBranchName,
			),
			"\n\n",
		)

		// Required checks section
		_, _ = fmt.Fprint(os.Stderr, "Required Checks\n")
		requiredCheckStatuses := query.GithubRepository.PullRequest.RequiredCheckStatuses
		for index := range requiredCheckStatuses {
			result := requiredCheckStatuses[index].Result
			requiredCheckName := requiredCheckStatuses[index].RequiredCheck.Pattern
			_, _ = fmt.Fprint(
				os.Stderr,
				indent,
				emojiForRequiredCheckResult(string(result)),
				" ",
				colors.UserInput(requiredCheckName),
				"\n",
			)
		}

		return nil
	},
}

func getQueryVariables() (map[string]interface{}, error) {
	repo, err := getRepo()
	if err != nil {
		return nil, err
	}

	db, err := getDB(repo)
	if err != nil {
		return nil, err
	}

	tx := db.ReadTx()

	currentBranchName, err := repo.CurrentBranchName()
	if err != nil {
		return nil, err
	}

	branch, _ := tx.Branch(currentBranchName)

	if branch.PullRequest == nil {
		return nil, errors.New(
			"this branch has no associated pull request (run `av pr create` to create one)",
		)
	}

	prNumber := branch.PullRequest.Number
	repository, exists := tx.Repository()
	if !exists {
		return nil, actions.ErrRepoNotInitialized
	}

	var variables = map[string]interface{}{
		"repoOwner": graphql.String(repository.Owner),
		"repoName":  graphql.String(repository.Name),
		"prNumber":  graphql.Int(prNumber),
	}
	return variables, nil
}

func emojiForRequiredCheckResult(result string) string {
	switch result {
	case "SUCCESS":
		return "\u2705"
	case "FAILURE":
		return "\u274C"
	default:
		return "\u231B"
	}
}
