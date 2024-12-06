package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/avgql"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/cleanup"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/shurcooL/githubv4"
	"github.com/shurcooL/graphql"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var prFlags struct {
	Draft     bool
	Force     bool
	NoPush    bool
	Title     string
	Body      string
	Edit      bool
	Reviewers []string
	Queue     bool
	All       bool
	Current   bool
}

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Create a pull request for the current branch",
	Long: strings.TrimSpace(`
Create a pull request for the current branch.

Examples:
  Create a PR with an empty body:
    $ av pr --title "My PR"

  Create a pull request, specifying the body of the PR from standard input.
    $ av pr --title "Implement fancy feature" --body - <<EOF
    > Implement my very fancy feature.
    > Can you please review it?
    > EOF

  Create a pull request, assigning reviewers:
    $ av pr --reviewers "example,@example-org/example-team"

  Create pull requests for every branch in the stack:
	$ av pr --all
`),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) (reterr error) {
		if prFlags.Queue {
			if prFlags.Draft ||
				prFlags.Force ||
				prFlags.NoPush ||
				prFlags.Title != "" ||
				prFlags.Body != "" ||
				prFlags.Edit ||
				prFlags.Reviewers != nil {

				return errors.New("cannot use other flags with --queue")
			}
			return queue()
		}

		if prFlags.All {
			if prFlags.Force ||
				prFlags.NoPush ||
				prFlags.Title != "" ||
				prFlags.Body != "" ||
				prFlags.Edit ||
				prFlags.Reviewers != nil ||
				prFlags.Queue {

				return errors.New("can only use --current and --draft with --all")
			}

			return submitAll(prFlags.Current, prFlags.Draft)
		}

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

		body := prFlags.Body
		// Special case: ready body from stdin
		if prFlags.Body == "-" {
			bodyBytes, err := io.ReadAll(os.Stdin)
			if err != nil {
				return errors.Wrap(err, "failed to read body from stdin")
			}
			prFlags.Body = string(bodyBytes)
		}

		draft := config.Av.PullRequest.Draft
		if cmd.Flags().Changed("draft") {
			draft = prFlags.Draft
		}

		ctx := context.Background()
		res, err := actions.CreatePullRequest(
			ctx, repo, client, tx,
			actions.CreatePullRequestOpts{
				BranchName: branchName,
				Title:      prFlags.Title,
				Body:       body,
				NoPush:     prFlags.NoPush,
				Force:      prFlags.Force,
				Draft:      draft,
				Edit:       prFlags.Edit,
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
		if len(prFlags.Reviewers) > 0 {
			if err := actions.AddPullRequestReviewers(ctx, client, res.Pull.ID, prFlags.Reviewers); err != nil {
				return err
			}
		}

		if config.Av.PullRequest.WriteStack {
			stackBranches, err := meta.StackBranches(tx, branchName)
			if err != nil {
				return err
			}

			return actions.UpdatePullRequestsWithStack(ctx, client, tx, stackBranches)
		}

		return nil
	},
}

func submitAll(current bool, draft bool) error {
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
	if err != nil {
		return err
	}

	var branchesToSubmit []string
	if current {
		previousBranches, err := meta.PreviousBranches(tx, currentBranch)
		if err != nil {
			return err
		}
		branchesToSubmit = append(branchesToSubmit, previousBranches...)
		branchesToSubmit = append(branchesToSubmit, currentBranch)
	} else {
		branchesToSubmit = currentStackBranches
	}

	if !current {
		subsequentBranches := meta.SubsequentBranches(tx, currentBranch)
		branchesToSubmit = append(branchesToSubmit, subsequentBranches...)
	}

	// ensure pull requests for each branch in the stack
	createdPullRequestPermalinks := []string{}
	ctx := context.Background()
	client, err := getGitHubClient()
	if err != nil {
		return err
	}
	for _, branchName := range branchesToSubmit {
		// TODO: should probably commit database after every call to this
		// since we're just syncing state from GitHub

		draft := config.Av.PullRequest.Draft || draft

		result, err := actions.CreatePullRequest(
			ctx, repo, client, tx,
			actions.CreatePullRequestOpts{
				BranchName:    branchName,
				Draft:         draft,
				NoOpenBrowser: true,
			},
		)
		if err != nil {
			return err
		}
		if result.Created {
			createdPullRequestPermalinks = append(
				createdPullRequestPermalinks,
				result.Branch.PullRequest.Permalink,
			)
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

	if config.Av.PullRequest.WriteStack {
		if err = actions.UpdatePullRequestsWithStack(ctx, client, tx, currentStackBranches); err != nil {
			return err
		}
	}

	if config.Av.PullRequest.OpenBrowser {
		for _, createdPullRequestPermalink := range createdPullRequestPermalinks {
			actions.OpenPullRequestInBrowser(createdPullRequestPermalink)
		}
	}

	return nil
}

func queue() error {
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
			"this branch has no associated pull request (run 'av pr' to create one)",
		)
	}

	prNumber := branch.PullRequest.Number
	repository := tx.Repository()

	var variables = map[string]interface{}{
		"repoOwner": graphql.String(repository.Owner),
		"repoName":  graphql.String(repository.Name),
		// prNumber is int64 graphql expects in32, we should not have more than 2^31-1 PRs
		"prNumber": graphql.Int(prNumber), //nolint:gosec
	}

	// I have a feeling this would be better written inside of av/internals
	client, err := avgql.NewClient()
	if err != nil {
		return err
	}

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
	fmt.Fprint(
		os.Stderr,
		"Queued pull request ", colors.UserInput(branch.PullRequest.Permalink), ".\n",
	)

	return nil
}

func init() {
	prCmd.Flags().BoolVar(
		&prFlags.Draft, "draft", false,
		"create the pull request in draft mode",
	)
	prCmd.Flags().BoolVar(
		&prFlags.Force, "force", false,
		"force creation of a pull request even if there is already a pull request associated with this branch",
	)
	prCmd.Flags().BoolVar(
		&prFlags.NoPush, "no-push", false,
		"don't push the branch to the remote repository before creating the pull request",
	)
	prCmd.Flags().StringVarP(
		&prFlags.Title, "title", "t", "",
		"title of the pull request to create",
	)
	prCmd.Flags().StringVarP(
		&prFlags.Body, "body", "b", "",
		"body of the pull request to create (a value of - will read from stdin)",
	)
	prCmd.Flags().BoolVar(
		&prFlags.Edit, "edit", false,
		"edit the pull request title and description before submitting even if the pull request already exists",
	)
	prCmd.Flags().StringSliceVar(
		&prFlags.Reviewers, "reviewers", nil,
		"add reviewers to the pull request (can be usernames or team names)",
	)
	prCmd.Flags().BoolVar(
		&prFlags.Queue, "queue", false,
		"queue an existing pull request for the current branch",
	)
	prCmd.Flags().BoolVar(
		&prFlags.All, "all", false,
		"create pull requests for every branch in stack (up to current branch with --current)",
	)
	prCmd.Flags().BoolVar(
		&prFlags.Current, "current", false,
		"create pull requests up to the current branch")
	_ = prCmd.Flags().MarkHidden("current")

	deprecatedCreateCmd := deprecateCommand(*prCmd, "av create", "create")
	deprecatedCreateCmd.Hidden = true

	prCmd.AddCommand(
		deprecatedCreateCmd,
		prQueueCmd,
		prStatusCmd,
	)

}
