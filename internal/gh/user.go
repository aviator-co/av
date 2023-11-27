package gh

import (
	"context"

	"emperror.dev/errors"
	"github.com/shurcooL/githubv4"
)

type User struct {
	ID    githubv4.ID `graphql:"id"`
	Login string      `graphql:"login"`
}

// User returns information about the given user.
func (c *Client) User(ctx context.Context, login string) (*User, error) {
	var query struct {
		User User `graphql:"user(login: $login)"`
	}
	if err := c.query(ctx, &query, map[string]any{
		"login": githubv4.String(login),
	}); err != nil {
		return nil, err
	}
	if query.User.ID == "" {
		return nil, errors.Errorf("GitHub user %q not found", login)
	}
	return &query.User, nil
}
