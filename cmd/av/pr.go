package main

import (
	"context"
	"io/ioutil"
	"os"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/config"
	"github.com/spf13/cobra"
)

var prCmd = &cobra.Command{
	Use: "pr",
}

var prCreateFlags struct {
	Base   string
	Draft  bool
	Force  bool
	NoPush bool
	Title  string
	Body   string
	Edit   bool
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
	RunE: func(cmd *cobra.Command, args []string) (reterr error) {
		repo, err := getRepo()
		if err != nil {
			return err
		}
		branchName, err := repo.CurrentBranchName()
		if err != nil {
			return errors.WrapIf(err, "failed to determine current branch")
		}
		client, err := getClient(config.Av.GitHub.Token)
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
			bodyBytes, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				return errors.Wrap(err, "failed to read body from stdin")
			}
			prCreateFlags.Body = string(bodyBytes)
		}

		if _, err := actions.CreatePullRequest(
			context.Background(), repo, client, tx,
			actions.CreatePullRequestOpts{
				BranchName: branchName,
				Title:      prCreateFlags.Title,
				Body:       body,
				NoPush:     prCreateFlags.NoPush,
				Force:      prCreateFlags.Force,
				// TODO: this means we can't override with --draft=false if the
				//       config has draft=true. We need to figure out how to
				//       unify config and flags (the latter should always
				//       override the former).
				Draft: prCreateFlags.Draft || config.Av.PullRequest.Draft,
				Edit:  prCreateFlags.Edit,
			},
		); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
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
}
