package main

import (
	"context"
	"fmt"
	"os"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/gh"

	"github.com/aviator-co/av/internal/avgql"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:          "auth",
	Short:        "Check user authentication status",
	SilenceUsage: true,
	Args:         cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		if err := checkAviatorAuthStatus(); err != nil {
			fmt.Fprintln(os.Stderr, colors.Warning(err.Error()))
		}
		if err := checkGitHubAuthStatus(); err != nil {
			fmt.Fprintln(os.Stderr, colors.Failure(err.Error()))
		}
	},
}

func checkAviatorAuthStatus() error {
	avClient, err := avgql.NewClient()
	if err != nil {
		return err
	}

	var query struct{ avgql.ViewerSubquery }
	if err := avClient.Query(context.Background(), &query, nil); err != nil {
		if avgql.IsHTTPUnauthorized(err) {
			return errors.New(
				"You are not logged in to Aviator. Please verify that your API token is correct.",
			)
		}
		return errors.Wrap(err, "Failed to query Aviator")
	}

	fmt.Fprint(os.Stderr,
		"Logged in to Aviator as ", colors.UserInput(query.Viewer.FullName),
		" (", colors.UserInput(query.Viewer.Email), ").\n",
	)
	return nil
}

func checkGitHubAuthStatus() error {
	ghClient, err := getGitHubClient()
	if err != nil {
		return err
	}

	viewer, err := ghClient.Viewer(context.Background())
	if err != nil {
		// GitHub API returns 401 Unauthorized if the token is invalid or
		// expired.
		if gh.IsHTTPUnauthorized(err) {
			return errors.New(
				"You are not logged in to GitHub. Please verify that your API token is correct.",
			)
		}
		return errors.Wrap(err, "Failed to query GitHub")
	}

	fmt.Fprint(os.Stderr,
		"Logged in to GitHub as ", colors.UserInput(viewer.Name),
		" (", colors.UserInput(viewer.Login), ").\n",
	)
	return nil
}

func init() {
	// deprecated 'av auth status', hidden to avoid it showing up in 'av auth --help'
	// since that is the new command name
	deprecatedAuthStatus := deprecateCommand(*authCmd, "av auth", "status")
	deprecatedAuthStatus.Hidden = true

	authCmd.AddCommand(
		deprecatedAuthStatus,
	)
}
