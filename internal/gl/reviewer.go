package gl

import (
	"context"

	"emperror.dev/errors"
)

// RequestReviews requests reviews from users for a merge request.
// GitLab doesn't have the exact same review request system as GitHub,
// but we can assign reviewers to the merge request.
func (c *Client) RequestReviews(ctx context.Context, projectPath string, iid int64, reviewerUsernames []string) error {
	if len(reviewerUsernames) == 0 {
		return nil // Nothing to do
	}

	// In GitLab, we assign reviewers rather than "request" reviews
	var mutation struct {
		MergeRequestSetReviewers struct {
			MergeRequest MergeRequest `graphql:"mergeRequest"`
			Errors       []string     `graphql:"errors"`
		} `graphql:"mergeRequestSetReviewers(input: $input)"`
	}

	variables := map[string]any{
		"input": map[string]any{
			"projectPath":       projectPath,
			"iid":               iid,
			"reviewerUsernames": reviewerUsernames,
		},
	}

	if err := c.mutate(ctx, &mutation, variables); err != nil {
		return WrapError(err, "request reviews")
	}

	if len(mutation.MergeRequestSetReviewers.Errors) > 0 {
		return errors.Errorf("failed to set reviewers: %v", mutation.MergeRequestSetReviewers.Errors)
	}

	return nil
}

// GetMergeRequestReviewers gets the current reviewers for a merge request
func (c *Client) GetMergeRequestReviewers(ctx context.Context, projectPath string, iid int64) ([]*User, error) {
	var query struct {
		Project struct {
			MergeRequest struct {
				Reviewers struct {
					Nodes []User `graphql:"nodes"`
				} `graphql:"reviewers"`
			} `graphql:"mergeRequest(iid: $iid)"`
		} `graphql:"project(fullPath: $projectPath)"`
	}

	variables := map[string]any{
		"projectPath": projectPath,
		"iid":         iid,
	}

	if err := c.query(ctx, &query, variables); err != nil {
		return nil, WrapError(err, "get merge request reviewers")
	}

	result := make([]*User, len(query.Project.MergeRequest.Reviewers.Nodes))
	for i, reviewer := range query.Project.MergeRequest.Reviewers.Nodes {
		result[i] = &reviewer
	}

	return result, nil
}