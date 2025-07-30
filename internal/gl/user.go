package gl

import (
	"context"

	"emperror.dev/errors"
)

type User struct {
	ID       GitLabID `graphql:"id"`
	Username string   `graphql:"username"`
	Name     string   `graphql:"name"`
	Email    string   `graphql:"email"`
	WebURL   string   `graphql:"webUrl"`
}

func (c *Client) GetCurrentUser(ctx context.Context) (*User, error) {
	var query struct {
		CurrentUser User `graphql:"currentUser"`
	}

	if err := c.query(ctx, &query, nil); err != nil {
		return nil, WrapError(err, "get current user")
	}

	if query.CurrentUser.ID == "" {
		return nil, errors.New("failed to get current user information")
	}

	return &query.CurrentUser, nil
}

func (c *Client) GetUser(ctx context.Context, username string) (*User, error) {
	var query struct {
		User User `graphql:"user(username: $username)"`
	}

	variables := map[string]any{
		"username": username,
	}

	if err := c.query(ctx, &query, variables); err != nil {
		return nil, WrapError(err, "get user")
	}

	if query.User.ID == "" {
		return nil, errors.Errorf("user not found: %s", username)
	}

	return &query.User, nil
}