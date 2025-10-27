package uiutils

import (
	"fmt"

	"emperror.dev/errors"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
)

var (
	ErrNoGitHubToken    = errors.Sentinel("No GitHub token is set (do you need to configure one?).")
	ErrParentNotAdopted = errors.Sentinel("Parent not adopted")
)

const noGitHubToken = `# ERROR: No GitHub Token

` + "`av`" + ` needs a GitHub API token to interact with the repository. There are two ways to provide a token:

1. (Easy) Use [GitHub CLI](https://cli.github.com) to authenticate with GitHub. Run ` + "`gh auth login`" + ` to authenticate.
2. Create a Personal Access Token on GitHub and set it in the config. See [av configuration doc](https://docs.aviator.co/aviator-cli/configuration#github-personal-access-token).

We couldn't find the GitHub CLI setup nor a Personal Access Token in the config. Please set up the token and try again.
`

const parentNotAdopted = `# ERROR: Parent branch is not adopted to ` + "`av`" + `

` + "`av`" + ` keeps metadata internally to keep track of branch relationships. If a branch is
created via ` + "`git`" + ` command, ` + "`av`" + ` doesn't have such metadata for that branch.

` + "`av adopt`" + ` is a command to adopt a ` + "`git`" + ` created branch to ` + "`av`" + `.
Please run ` + "`av adopt`" + ` to adopt the parent branch first.
`

func RenderError(err error) string {
	var style string
	if lipgloss.HasDarkBackground() {
		style = styles.DarkStyle
	} else {
		style = styles.LightStyle
	}
	var markdownText string
	if errors.Is(err, ErrNoGitHubToken) {
		markdownText = noGitHubToken
	} else if errors.Is(err, ErrParentNotAdopted) {
		markdownText = parentNotAdopted
	}

	if markdownText != "" {
		if out, rerr := glamour.Render(markdownText, style); rerr == nil {
			return out
		}
		// If there's an error, fallback to the plaintext message.
	}
	// This is a placeholder for a more sophisticated error renderer.
	// For now, we just print the error message.
	return fmt.Sprintf("error: %s\n", err)
}
