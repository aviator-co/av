package gh

import "context"

type Viewer struct {
	Name  string `graphql:"name"`
	Login string `graphql:"login"`
}

func (c *Client) Viewer(ctx context.Context) (*Viewer, error) {
	var query struct {
		Viewer Viewer `graphql:"viewer"`
	}
	err := c.query(ctx, &query, nil)
	if err != nil {
		return nil, err
	}
	return &query.Viewer, nil
}
