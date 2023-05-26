package gh

import (
	"context"
	"strings"

	"emperror.dev/errors"
	"github.com/shurcooL/githubv4"
)

type Repository struct {
	ID    string
	Owner struct {
		Login string
	}
	Name string
}

func (c *Client) GetRepositoryBySlug(ctx context.Context, slug string) (*Repository, error) {
	owner, name, ok := strings.Cut(slug, "/")
	if !ok {
		return nil, errors.Errorf(
			"unable to parse repository slug (expected <owner>/<repo>): %q",
			slug,
		)
	}

	var query struct {
		Repository Repository `graphql:"repository(owner: $owner, name: $name)"`
	}
	err := c.query(ctx, &query, map[string]interface{}{
		"owner": githubv4.String(owner),
		"name":  githubv4.String(name),
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to fetch repository from GitHub")
	}

	return &query.Repository, nil
}
