package gh

import (
	"context"
	"fmt"
	"strings"

	"emperror.dev/errors"
	"github.com/shurcooL/githubv4"
)

type PullRequest struct {
	ID     string
	Number int64
	Author struct {
		Login string
	}
	HeadRefName         string
	HeadRefOID          string
	BaseRefName         string
	IsDraft             bool
	Mergeable           githubv4.MergeableState
	Merged              bool
	Permalink           string
	State               githubv4.PullRequestState
	Title               string
	Body                string
	PRIVATE_MergeCommit struct {
		Oid string
	} `graphql:"mergeCommit"`
	PRIVATE_TimelineItems struct {
		Nodes []struct {
			ClosedEvent struct {
				Closer struct {
					Commit struct {
						Oid string
					} `graphql:"... on Commit"`
				}
			} `graphql:"... on ClosedEvent"`
		}
	} `graphql:"timelineItems(last: 10, itemTypes: CLOSED_EVENT)"`
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

func (p *PullRequest) GetMergeCommit() string {
	if p.State == githubv4.PullRequestStateOpen {
		return ""
	} else if p.State == githubv4.PullRequestStateMerged {
		return p.PRIVATE_MergeCommit.Oid
	} else if p.State == githubv4.PullRequestStateClosed && len(p.PRIVATE_TimelineItems.Nodes) != 0 {
		return p.PRIVATE_TimelineItems.Nodes[0].ClosedEvent.Closer.Commit.Oid
	}
	return ""
}

type PullRequestOpts struct {
	Owner  string
	Repo   string
	Number int64
}

func (c *Client) PullRequest(ctx context.Context, id string) (*PullRequest, error) {
	var query struct {
		Node struct {
			PullRequest PullRequest `graphql:"... on PullRequest"`
		} `graphql:"node(id: $id)"`
	}
	if err := c.query(ctx, &query, map[string]interface{}{
		"id": githubv4.ID(id),
	}); err != nil {
		return nil, errors.Wrap(err, "failed to query pull request")
	}
	if query.Node.PullRequest.ID == "" {
		return nil, errors.Errorf("pull request %q not found", id)
	}
	return &query.Node.PullRequest, nil
}

type GetPullRequestsInput struct {
	// REQUIRED
	Owner string
	Repo  string
	// OPTIONAL
	HeadRefName string
	BaseRefName string
	States      []githubv4.PullRequestState
	First       int64
	After       string
}

type GetPullRequestsPage struct {
	PageInfo
	PullRequests []PullRequest
}

func (c *Client) GetPullRequests(
	ctx context.Context,
	input GetPullRequestsInput,
) (*GetPullRequestsPage, error) {
	if input.First == 0 {
		input.First = 50
	}
	var query struct {
		Repository struct {
			PullRequests struct {
				Nodes    []PullRequest
				PageInfo PageInfo
			} `graphql:"pullRequests(states: $states, headRefName: $headRefName, baseRefName: $baseRefName, first: $first, after: $after)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}
	if err := c.query(ctx, &query, map[string]interface{}{
		"owner":       githubv4.String(input.Owner),
		"repo":        githubv4.String(input.Repo),
		"headRefName": nullable(githubv4.String(input.HeadRefName)),
		"baseRefName": nullable(githubv4.String(input.BaseRefName)),
		"states":      &input.States,
		"first":       githubv4.Int(input.First),
		"after":       nullable(githubv4.String(input.After)),
	}); err != nil {
		return nil, errors.Wrap(err, "failed to query pull requests")
	}
	return &GetPullRequestsPage{
		PageInfo:     query.Repository.PullRequests.PageInfo,
		PullRequests: query.Repository.PullRequests.Nodes,
	}, nil
}

func (c *Client) CreatePullRequest(
	ctx context.Context,
	input githubv4.CreatePullRequestInput,
) (*PullRequest, error) {
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

func (c *Client) UpdatePullRequest(
	ctx context.Context,
	input githubv4.UpdatePullRequestInput,
) (*PullRequest, error) {
	var mutation struct {
		UpdatePullRequest struct {
			PullRequest PullRequest
		} `graphql:"updatePullRequest(input: $input)"`
	}
	if err := c.mutate(ctx, &mutation, input, nil); err != nil {
		return nil, errors.Wrap(err, "failed to update pull request: github error")
	}
	return &mutation.UpdatePullRequest.PullRequest, nil
}

// RequestReviews requests reviews from the given users on the given pull
// request.
func (c *Client) RequestReviews(
	ctx context.Context,
	input githubv4.RequestReviewsInput,
) (*PullRequest, error) {
	if input.Union == nil {
		// Add reviewers instead of replacing them by default.
		input.Union = Ptr[githubv4.Boolean](true)
	}
	var mutation struct {
		RequestReviews struct {
			PullRequest PullRequest
		} `graphql:"requestReviews(input: $input)"`
	}
	if err := c.mutate(ctx, &mutation, input, nil); err != nil {
		return nil, errors.Wrap(err, "failed to request pull request reviews")
	}
	return &mutation.RequestReviews.PullRequest, nil
}

func (c *Client) ConvertPullRequestToDraft(ctx context.Context, id string) (*PullRequest, error) {
	var mutation struct {
		ConvertPullRequestToDraft struct {
			PullRequest PullRequest
		} `graphql:"convertPullRequestToDraft(input: $input)"`
	}
	if err := c.mutate(ctx, &mutation, githubv4.ConvertPullRequestToDraftInput{PullRequestID: id}, nil); err != nil {
		return nil, errors.Wrap(err, "failed to convert pull request to draft: github error")
	}
	return &mutation.ConvertPullRequestToDraft.PullRequest, nil
}

func (c *Client) MarkPullRequestReadyForReview(
	ctx context.Context,
	id string,
) (*PullRequest, error) {
	var mutation struct {
		MarkPullRequestReadyForReview struct {
			PullRequest PullRequest
		} `graphql:"markPullRequestReadyForReview(input: $input)"`
	}
	if err := c.mutate(ctx, &mutation, githubv4.MarkPullRequestReadyForReviewInput{PullRequestID: id}, nil); err != nil {
		return nil, errors.Wrap(err, "failed to mark pull request ready for review: github error")
	}
	return &mutation.MarkPullRequestReadyForReview.PullRequest, nil
}

type AddIssueLabelInput struct {
	// The owner of the GitHub repository.
	Owner string
	// The name of the GitHub repository.
	Repo string
	// The number of the issue or pull request to add a label to.
	Number int64
	// The names of the labels to add to the issue. This will implicitly create
	// a label on the repository if the label doesn't already exist (this is the
	// main reason we use the REST API for this call).
	LabelNames []string
}

// AddIssueLabels adds labels to an issue (or pull request, since in GitHub
// a pull request is a superset of an issue).
func (c *Client) AddIssueLabels(ctx context.Context, input AddIssueLabelInput) error {
	// Working with labels is still kind of a pain in the GitHub GraphQL API
	// (you have to add labels by node id, not label name, and there's no way to
	// create labels from the GraphQL API), so we just use v3/REST here.
	req := struct {
		Labels []string `json:"labels"`
	}{
		Labels: input.LabelNames,
	}
	endpoint := fmt.Sprintf("/repos/%s/%s/issues/%d", input.Owner, input.Repo, input.Number)
	if err := c.restPost(ctx, endpoint, req, nil); err != nil {
		return errors.Wrap(err, "failed to add labels")
	}
	return nil
}

type RepoPullRequestOpts struct {
	Owner  string
	Repo   string
	First  int64
	After  string
	States []githubv4.PullRequestState
}

type RepoPullRequestsResponse struct {
	PageInfo
	TotalCount   int64
	PullRequests []PullRequest
}

func (c *Client) RepoPullRequests(
	ctx context.Context,
	opts RepoPullRequestOpts,
) (RepoPullRequestsResponse, error) {
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
