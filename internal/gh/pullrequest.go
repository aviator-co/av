package gh

import (
	"context"
	"emperror.dev/errors"
	"github.com/shurcooL/githubv4"
	"strings"
)

type PullRequest struct {
	ID     string
	Number int64
	Author struct {
		Login string
	}
	HeadRefName string
	HeadRefOID  string
	BaseRefName string
	IsDraft     bool
	Mergeable   githubv4.MergeableState
	Merged      bool
	Permalink   string
	State       githubv4.PullRequestState
	Title       string
}

func (p *PullRequest) HeadBranchName() string {
	// Note: GH sometimes includes the "refs/heads/" prefix and sometimes it doesn't.
	// I think(?) it might just return exactly what is given to the API during
	// creation.
	return strings.TrimPrefix(p.HeadRefName, "refs/heads/")
}

func (p *PullRequest) BaseBranchName() string {
	// See comment in HeadBranchName above.
	return strings.TrimPrefix(p.BaseRefName, "refs/heads/")
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

type RepoPullRequestOpts struct {
	Owner  string
	Repo   string
	First  int64
	After  string
	States []githubv4.PullRequestState
}

type PageInfo struct {
	EndCursor       string
	HasNextPage     bool
	HasPreviousPage bool
	StartCursor     string
}

type RepoPullRequestsResponse struct {
	PageInfo
	TotalCount   int64
	PullRequests []PullRequest
}

func (c *Client) RepoPullRequests(ctx context.Context, opts RepoPullRequestOpts) (RepoPullRequestsResponse, error) {
	var query struct {
		Repository struct {
			PullRequests struct {
				TotalCount int64
				PageInfo   PageInfo
				Nodes      []PullRequest
			} `graphql:"pullRequests(states: $states, first: $first, after: $after)"`
		} `graphql:"repository(owner:$owner, name:$repo)"`
	}

	if opts.First == 0 {
		opts.First = 100
	}
	vars := map[string]any{
		"owner":  githubv4.String(opts.Owner),
		"repo":   githubv4.String(opts.Repo),
		"first":  githubv4.Int(opts.First),
		"after":  nullable(githubv4.String(opts.After)),
		"states": opts.States,
	}
	if opts.After != "" {
		vars["after"] = githubv4.String(opts.After)
	}
	if len(opts.States) > 0 {
		vars["states"] = opts.States
	}
	if err := c.query(ctx, &query, vars); err != nil {
		return RepoPullRequestsResponse{}, errors.Wrap(err, "failed to query pull requests")
	}
	return RepoPullRequestsResponse{
		PageInfo:     query.Repository.PullRequests.PageInfo,
		TotalCount:   query.Repository.PullRequests.TotalCount,
		PullRequests: query.Repository.PullRequests.Nodes,
	}, nil
}
