package main

import (
	"context"
	"fmt"
	"os"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/gh"

	"github.com/aviator-co/av/internal/avgql"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/spf13/cobra"
)

var authStatusCmd = &cobra.Command{
	Use:          "status",
	Short:        "Show info about the logged in user",
	SilenceUsage: true,
	Args:         cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		exitCode := 0
		if err := checkAviatorAuthStatus(); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, colors.Failure(err.Error()))
			exitCode = 1
		}
		if err := checkGitHubAuthStatus(); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, colors.Failure(err.Error()))
			exitCode = 1
		}
		if exitCode != 0 {
			return actions.ErrExitSilently{ExitCode: exitCode}
		}
		return nil
	},
}

func checkAviatorAuthStatus() error {
	avClient, err := avgql.NewClient()
	if err != nil {
		return err
	}

	var query struct{ avgql.ViewerSubquery }
	if err := avClient.Query(context.Background(), &query, nil); err != nil {
		return err
	}
	if err := query.CheckViewer(); err != nil {
		return err
	}

	_, _ = fmt.Fprint(os.Stderr,
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

	_, _ = fmt.Fprint(os.Stderr,
		"Logged in to GitHub as ", colors.UserInput(viewer.Name),
		" (", colors.UserInput(viewer.Login), ").\n",
	)
	return nil
}
