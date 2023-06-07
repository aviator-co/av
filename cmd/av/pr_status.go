package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/aviator-co/av/internal/avgql"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/shurcooL/graphql"
	"github.com/spf13/cobra"
)

var query struct {
	GithubRepository struct {
		PullRequest struct {
			Title        graphql.String
			Status       graphql.String
			StatusReason graphql.String
			Author       struct {
				Login graphql.String
			}
			CreatedAt             graphql.String
			QueuedAt              graphql.String
			MergedAt              graphql.String
			BaseBranchName        graphql.String
			HeadBranchName        graphql.String
			RequiredCheckStatuses []struct {
				RequiredCheck struct {
					Pattern graphql.String
				}
				Result    graphql.String
				CheckRuns []struct {
					Check struct {
						Name graphql.String
					}
					Status     graphql.String
					Conclusion graphql.String
				}
			}
		} `graphql:"pullRequest(number: $prNumber)"`
	} `graphql:"githubRepository(owner: $repoOwner, name:$repoName)"`
}

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

		err = client.Query(context.Background(), &query, variables)
		if err != nil {
			return err
		}

		indent := "    "
		prStatus := query.GithubRepository.PullRequest.Status

		// Print PR info
		_, _ = fmt.Fprint(
			os.Stderr,
			"#162 ",
			colors.UserInput(query.GithubRepository.PullRequest.Title),
			"\n",
		)
		_, _ = fmt.Fprint(os.Stderr, indent, "Status: ", colors.UserInput(prStatus), "\n")

		if prStatus == "PENDING" || prStatus == "BLOCKED" {
			_, _ = fmt.Fprint(
				os.Stderr,
				indent,
				"Status reason: ",
				colors.UserInput(query.GithubRepository.PullRequest.StatusReason),
				"\n",
			)
		}

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
			colors.UserInput(query.GithubRepository.PullRequest.CreatedAt),
			"\n",
		)

		if prStatus == "QUEUED" {
			_, _ = fmt.Fprint(
				os.Stderr,
				indent,
				"Queued at: ",
				colors.UserInput(query.GithubRepository.PullRequest.QueuedAt),
				"\n",
			)
		}
		if prStatus == "MERGED" {
			_, _ = fmt.Fprint(
				os.Stderr,
				indent,
				"Merged at: ",
				colors.UserInput(query.GithubRepository.PullRequest.MergedAt),
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

	branch, exists := tx.Branch(currentBranchName)
	if !exists {
		return nil, errors.New("could not find current branch")
	}

	if branch.PullRequest == nil {
		return nil, errors.New("pull request does not exist")
	}

	// prNumber := branch.PullRequest.Number
	repository, _ := tx.Repository()
	var variables = map[string]interface{}{
		"repoOwner": graphql.String(repository.Owner),
		"repoName":  graphql.String(repository.Name),
		"prNumber":  graphql.Int(2897),
	}
	return variables, nil
}

func emojiForRequiredCheckResult(result string) string {
	if result == "SUCCESS" {
		return "\u2705"
	} else if result == "FAILURE" {
		return "\u274C"
	} else {
		return "\u231B"
	}
}
