package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/aviator-co/av/internal/avgql"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/timeutils"
	"github.com/shurcooL/githubv4"
	"github.com/shurcooL/graphql"
	"github.com/spf13/cobra"
)

var prStatusCmd = &cobra.Command{
	Use:          "status",
	Short:        "Get the status of the associated pull request",
	SilenceUsage: true,
	Args:         cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		variables, err := getQueryVariables()
		if err != nil {
			return err
		}

		client, err := avgql.NewClient()
		if err != nil {
			return err
		}

		var query struct {
			avgql.ViewerSubquery
			GithubRepository struct {
				PullRequest struct {
					Number       graphql.Int
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

					BotPullRequest struct {
						Number                graphql.Int
						RequiredCheckStatuses []struct {
							RequiredCheck struct {
								Pattern graphql.String
							}
							Result graphql.String
						}
					}
				} `graphql:"pullRequest(number: $prNumber)"`
			} `graphql:"githubRepository(owner: $repoOwner, name:$repoName)"`
		}
		if err := client.Query(context.Background(), &query, variables); err != nil {
			return err
		}
		if err := query.CheckViewer(); err != nil {
			return err
		}

		pr := query.GithubRepository.PullRequest
		if pr.Number == 0 {
			return errors.New("pull request not found")
		}

		// Print PR info
		indent := "    "
		fmt.Fprint(
			os.Stderr,
			"#",
			variables["prNumber"],
			" ",
			colors.UserInput(pr.Title),
			"\n",
		)
		fmt.Fprint(os.Stderr, indent, "Status: ", colors.UserInput(pr.Status))

		if pr.Status == "PENDING" || pr.Status == "BLOCKED" {
			fmt.Fprint(
				os.Stderr,
				indent,
				" (",
				colors.UserInput(query.GithubRepository.PullRequest.StatusReason),
				")",
			)
		}
		fmt.Fprint(os.Stderr, "\n")

		fmt.Fprint(
			os.Stderr,
			indent,
			"Author: ",
			colors.UserInput(query.GithubRepository.PullRequest.Author.Login),
			"\n",
		)
		fmt.Fprint(
			os.Stderr,
			indent,
			"Created at: ",
			colors.UserInput(
				timeutils.FormatLocal(query.GithubRepository.PullRequest.CreatedAt.Time),
			),
			"\n",
		)

		if pr.Status == "QUEUED" {
			fmt.Fprint(
				os.Stderr,
				indent,
				"Queued at: ",
				colors.UserInput(
					timeutils.FormatLocal(query.GithubRepository.PullRequest.QueuedAt.Time),
				),
				"\n",
			)
		}
		if pr.Status == "MERGED" {
			fmt.Fprint(
				os.Stderr,
				indent,
				"Merged at: ",
				colors.UserInput(
					timeutils.FormatLocal(query.GithubRepository.PullRequest.MergedAt.Time),
				),
				"\n",
			)
		}

		fmt.Fprint(
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
		fmt.Fprint(os.Stderr, "Required Checks\n")
		requiredCheckStatuses := query.GithubRepository.PullRequest.RequiredCheckStatuses
		for index := range requiredCheckStatuses {
			result := requiredCheckStatuses[index].Result
			requiredCheckName := requiredCheckStatuses[index].RequiredCheck.Pattern
			fmt.Fprint(
				os.Stderr,
				indent,
				emojiForRequiredCheckResult(string(result)),
				" ",
				colors.UserInput(requiredCheckName),
				"\n",
			)
		}

		// Get Bot Pull Request info
		botPullRequest := query.GithubRepository.PullRequest.BotPullRequest
		if botPullRequest.Number == 0 {
			// no bot pull request info
			return nil
		}

		fmt.Fprint(
			os.Stderr,
			"Bot Pull Request #",
			botPullRequest.Number,
			" Required Checks\n",
		)

		botRequiredCheckStatuses := query.GithubRepository.PullRequest.BotPullRequest.RequiredCheckStatuses
		for _, status := range botRequiredCheckStatuses {
			fmt.Fprint(
				os.Stderr,
				indent,
				emojiForRequiredCheckResult(string(status.Result)),
				" ",
				colors.UserInput(status.RequiredCheck.Pattern),
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
			"this branch has no associated pull request (run 'av pr' to create one)",
		)
	}

	prNumber := branch.PullRequest.Number
	repository := tx.Repository()
	var variables = map[string]interface{}{
		"repoOwner": graphql.String(repository.Owner),
		"repoName":  graphql.String(repository.Name),
		"prNumber":  graphql.Int(prNumber), //nolint:gosec
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
