package main

import (
	"fmt"
	"os"
	"context"

	"github.com/aviator-co/av/internal/gql"
	"github.com/shurcooL/graphql"
	"github.com/spf13/cobra"
)

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "check auth status",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := gql.GraphQLClient()

		var query struct {
			Viewer struct {
				Email graphql.String
			}
		}

		err := client.Query(context.Background(), &query, nil)
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			fmt.Printf("<ERROR: %v>\n", err)
			return err
		}

		if query.Viewer.Email == "" {
			fmt.Fprintln(os.Stderr, "Error: Could not find auth info, please verify that your API Token is correct.")
			return nil
		}

		_, _ = fmt.Fprintln(os.Stderr, "Logged in as: " + query.Viewer.Email)
		return nil
	},
}

