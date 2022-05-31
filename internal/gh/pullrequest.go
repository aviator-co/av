package gh

import (
	"context"
	"emperror.dev/errors"
	"github.com/shurcooL/githubv4"
)

type MergeableState string

const (
	MergeableStateUnknown     MergeableState = "UNKNOWN"
	MergeableStateMergeable   MergeableState = "MERGEABLE"
	MergeableStateConflicting MergeableState = "CONFLICTING"
)

type PullRequest struct {
	ID     string
	Number int64
	Author struct {
		Login string
	}
	HeadRefName string
	BaseRefName string
	IsDraft     bool
	Mergeable   MergeableState
	Merged      bool
	Permalink   string
}

type PullRequestOpts struct {
	Owner  string
	Repo   string
	Number int64
}

func (c *Client) PullRequest(ctx context.Context, opts PullRequestOpts) (*PullRequest, error) {
	var query struct {
		Repository struct {
			PullRequest PullRequest `graphql:"pullRequest(number: $number)"`
		} `graphql:"repository(owner:$owner, name:$repo)"`
	}
	if err := c.query(ctx, &query, map[string]interface{}{
		"owner":  githubv4.String(opts.Owner),
		"repo":   githubv4.String(opts.Repo),
		"number": githubv4.Int(opts.Number),
	}); err != nil {
		return nil, errors.WrapIff(err, "failed to query pull request #%d", opts.Number)
	}
	return &query.Repository.PullRequest, nil
}

func (c *Client) CreatePullRequest(ctx context.Context, input githubv4.CreatePullRequestInput) (*PullRequest, error) {
	var mutation struct {
		CreatePullRequest struct {
			PullRequest PullRequest
		} `graphql:"createPullRequest(input: $input)"`
	}
	if err := c.mutate(ctx, &mutation, input, nil); err != nil {
		return nil, errors.Wrap(err, "failed to create pull request: github error")
	}
	return &mutation.CreatePullRequest.PullRequest, nil
}
