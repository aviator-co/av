package gh

import (
	"context"

	"emperror.dev/errors"
	"github.com/shurcooL/githubv4"
)

type Team struct {
	ID   githubv4.ID `graphql:"id"`
	Name string      `graphql:"name"`
	Slug string      `graphql:"slug"`
}

// OrganizationTeam returns information about the given team in the given organization.
func (c *Client) OrganizationTeam(
	ctx context.Context,
	organizationLogin string,
	teamSlug string,
) (*Team, error) {
	var query struct {
		Organization struct {
			ID   githubv4.ID `graphql:"id"`
			Team Team        `graphql:"team(slug: $teamSlug)"`
		} `graphql:"organization(login: $organizationLogin)"`
	}
	if err := c.query(ctx, &query, map[string]any{
		"organizationLogin": githubv4.String(organizationLogin),
		"teamSlug":          githubv4.String(teamSlug),
	}); err != nil {
		return nil, err
	}
	if query.Organization.ID == "" {
		return nil, errors.Errorf("GitHub organization %q not found", organizationLogin)
	}
	if query.Organization.Team.ID == "" {
		return nil, errors.Errorf(
			"GitHub team %q not found within organization %q",
			teamSlug,
			organizationLogin,
		)
	}
	return &query.Organization.Team, nil
}
