package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/avgql"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/shurcooL/graphql"
	"github.com/spf13/cobra"
)

var authStatusCmd = &cobra.Command{
	Use:          "status",
	Short:        "check auth status",
	SilenceUsage: true,
	Args:         cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := avgql.NewClient()
		if err != nil {
			return err
		}

		var query struct {
			Viewer struct {
				Email graphql.String
			}
		}

		err = client.Query(context.Background(), &query, nil)
		if err != nil {
			return err
		}

		if query.Viewer.Email == "" {
			_, _ = fmt.Fprint(
				os.Stderr,
				colors.Failure(
					"You are not logged in. Please verify that your API token is correct.\n",
				),
			)
			return actions.ErrExitSilently{ExitCode: 1}
		}

		_, _ = fmt.Fprint(os.Stderr, "Logged in as ", colors.UserInput(query.Viewer.Email), ".\n")
		return nil
	},
}
