package main

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

const noGitHubToken = `# ERROR: No GitHub Token

` + "`av`" + ` needs a GitHub API token to interact with the repository. There are two ways to provide a token:

1. (Easy) Use [GitHub CLI](https://cli.github.com) to authenticate with GitHub. Run ` + "`gh auth login`" + ` to authenticate.
2. Create a Personal Access Token on GitHub and set it in the config. See [av configuration doc](https://docs.aviator.co/aviator-cli/configuration#github-personal-access-token).

We couldn't find the GitHub CLI setup nor a Personal Access Token in the config. Please set up the token and try again.
`

func renderError(err error) string {
	var style string
	if lipgloss.HasDarkBackground() {
		style = glamour.DarkStyle
	} else {
		style = glamour.LightStyle
	}
	if errors.Is(err, errNoGitHubToken) {
		if out, rerr := glamour.Render(noGitHubToken, style); rerr == nil {
			return out
		}
	}
	// This is a placeholder for a more sophisticated error renderer.
	// For now, we just print the error message.
	return fmt.Sprintf("error: %s\n", err)
}
