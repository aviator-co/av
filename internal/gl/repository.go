package gl

import (
	"context"
	"strings"

	"emperror.dev/errors"
)

type Repository struct {
	ID       GitLabID `graphql:"id"`
	Name     string   `graphql:"name"`
	FullPath string   `graphql:"fullPath"`
	Owner    struct {
		Username string `graphql:"username"`
	} `graphql:"owner"`
}

func (c *Client) GetRepositoryBySlug(ctx context.Context, slug string) (*Repository, error) {
	// GitLab uses fullPath instead of owner/name separation
	var query struct {
		Project Repository `graphql:"project(fullPath: $fullPath)"`
	}

	variables := map[string]any{
		"fullPath": slug,
	}

	if err := c.query(ctx, &query, variables); err != nil {
		return nil, WrapError(err, "get repository")
	}

	if query.Project.ID == "" {
		return nil, errors.Errorf("repository not found: %s", slug)
	}

	return &query.Project, nil
}

// GetRepositoryByID retrieves a repository by its GitLab ID.
func (c *Client) GetRepositoryByID(ctx context.Context, id string) (*Repository, error) {
	var query struct {
		Project Repository `graphql:"project(id: $id)"`
	}

	variables := map[string]any{
		"id": id,
	}

	if err := c.query(ctx, &query, variables); err != nil {
		return nil, WrapError(err, "get repository by ID")
	}

	if query.Project.ID == "" {
		return nil, errors.Errorf("repository not found with ID: %s", id)
	}

	return &query.Project, nil
}

// Helper function to extract owner/repo from GitLab's fullPath
func ParseFullPath(fullPath string) (owner, repo string, ok bool) {
	parts := strings.Split(fullPath, "/")
	if len(parts) < 2 {
		return "", "", false
	}
	
	// For nested groups, take the last part as repo name
	// and join the rest as the owner/group path
	repo = parts[len(parts)-1]
	owner = strings.Join(parts[:len(parts)-1], "/")
	return owner, repo, true
}