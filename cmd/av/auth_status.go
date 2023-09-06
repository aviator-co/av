package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aviator-co/av/internal/avgql"
	"github.com/aviator-co/av/internal/utils/colors"
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
			avgql.ViewerSubquery
		}

		if err := client.Query(context.Background(), &query, nil); err != nil {
			return err
		}
		if err := query.CheckViewer(); err != nil {
			return err
		}

		_, _ = fmt.Fprint(os.Stderr,
			"Logged in as ", colors.UserInput(query.Viewer.FullName),
			" (", colors.UserInput(query.Viewer.Email), ").\n",
		)
		return nil
	},
}
