package providers

import (
	"context"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/gh"
	"github.com/shurcooL/githubv4"
)

// GitHubClientWrapper wraps the GitHub client to implement our provider interface.
type GitHubClientWrapper struct {
	client   *gh.Client
	repoSlug string
}

func (w *GitHubClientWrapper) GetRepository(ctx context.Context, slug string) (*Repository, error) {
	repo, err := w.client.GetRepositoryBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	return &Repository{
		ID:       repo.ID,
		Owner:    repo.Owner.Login,
		Name:     repo.Name,
		FullName: repo.Owner.Login + "/" + repo.Name,
	}, nil
}

func (w *GitHubClientWrapper) GetPullRequest(ctx context.Context, id string) (*PullRequest, error) {
	pr, err := w.client.PullRequest(ctx, id)
	if err != nil {
		return nil, err
	}

	return w.convertGitHubPR(pr), nil
}

func (w *GitHubClientWrapper) CreatePullRequest(ctx context.Context, opts *CreatePullRequestOpts) (*PullRequest, error) {
	createOpts := &gh.CreatePullRequestOpts{
		Repository:  opts.Repository,
		Title:       opts.Title,
		Body:        opts.Body,
		HeadRefName: opts.HeadRefName,
		BaseRefName: opts.BaseRefName,
		Draft:       opts.IsDraft,
	}

	pr, err := w.client.CreatePullRequest(ctx, createOpts)
	if err != nil {
		return nil, err
	}

	// Request reviews if specified
	if len(opts.Reviewers) > 0 {
		if err := w.client.RequestReviews(ctx, pr.ID, opts.Reviewers, nil); err != nil {
			// Don't fail the entire operation if review requests fail
			// Just log and continue
		}
	}

	return w.convertGitHubPR(pr), nil
}

func (w *GitHubClientWrapper) UpdatePullRequest(ctx context.Context, opts *UpdatePullRequestOpts) (*PullRequest, error) {
	updateOpts := &gh.UpdatePullRequestOpts{
		ID: opts.ID,
	}

	if opts.Title != nil {
		updateOpts.Title = opts.Title
	}
	if opts.Body != nil {
		updateOpts.Body = opts.Body
	}
	if opts.BaseRefName != nil {
		updateOpts.BaseRefName = opts.BaseRefName
	}

	pr, err := w.client.UpdatePullRequest(ctx, updateOpts)
	if err != nil {
		return nil, err
	}

	return w.convertGitHubPR(pr), nil
}

func (w *GitHubClientWrapper) GetPullRequests(ctx context.Context, opts *GetPullRequestsOpts) ([]*PullRequest, error) {
	var state *githubv4.PullRequestState
	if opts.State != nil {
		switch *opts.State {
		case PullRequestStateOpen:
			state = &[]githubv4.PullRequestState{githubv4.PullRequestStateOpen}[0]
		case PullRequestStateClosed:
			state = &[]githubv4.PullRequestState{githubv4.PullRequestStateClosed}[0]
		case PullRequestStateMerged:
			state = &[]githubv4.PullRequestState{githubv4.PullRequestStateMerged}[0]
		}
	}

	getPRsOpts := &gh.GetPullRequestsOpts{
		Repository:  opts.Repository,
		State:       state,
		HeadRefName: opts.HeadRefName,
		BaseRefName: opts.BaseRefName,
	}

	prs, err := w.client.GetPullRequests(ctx, getPRsOpts)
	if err != nil {
		return nil, err
	}

	result := make([]*PullRequest, len(prs))
	for i, pr := range prs {
		result[i] = w.convertGitHubPR(pr)
	}

	return result, nil
}

func (w *GitHubClientWrapper) ConvertToDraft(ctx context.Context, id string) (*PullRequest, error) {
	pr, err := w.client.ConvertPullRequestToDraft(ctx, id)
	if err != nil {
		return nil, err
	}

	return w.convertGitHubPR(pr), nil
}

func (w *GitHubClientWrapper) MarkReadyForReview(ctx context.Context, id string) (*PullRequest, error) {
	pr, err := w.client.MarkPullRequestReadyForReview(ctx, id)
	if err != nil {
		return nil, err
	}

	return w.convertGitHubPR(pr), nil
}

func (w *GitHubClientWrapper) RequestReviews(ctx context.Context, id string, reviewers []string, teamReviewers []string) error {
	return w.client.RequestReviews(ctx, id, reviewers, teamReviewers)
}

func (w *GitHubClientWrapper) GetCurrentUser(ctx context.Context) (*User, error) {
	viewer, err := w.client.Viewer(ctx)
	if err != nil {
		return nil, err
	}

	return &User{
		ID:    viewer.ID,
		Login: viewer.Login,
		Name:  viewer.Name,
		Email: viewer.Email,
	}, nil
}

func (w *GitHubClientWrapper) GetUser(ctx context.Context, login string) (*User, error) {
	user, err := w.client.User(ctx, login)
	if err != nil {
		return nil, err
	}

	return &User{
		ID:    user.ID,
		Login: user.Login,
		Name:  user.Name,
		Email: user.Email,
	}, nil
}

func (w *GitHubClientWrapper) convertGitHubPR(pr *gh.PullRequest) *PullRequest {
	var state PullRequestState
	switch pr.State {
	case githubv4.PullRequestStateOpen:
		state = PullRequestStateOpen
	case githubv4.PullRequestStateClosed:
		state = PullRequestStateClosed
	case githubv4.PullRequestStateMerged:
		state = PullRequestStateMerged
	}

	converted := &PullRequest{
		ID:          pr.ID,
		Number:      pr.Number,
		Title:       pr.Title,
		Body:        pr.Body,
		State:       state,
		IsDraft:     pr.IsDraft,
		HeadRefName: pr.HeadRefName,
		BaseRefName: pr.BaseRefName,
		Permalink:   pr.Permalink,
		// Note: GitHub's time fields need to be converted if they're custom types
	}

	// Handle merge commit if available
	if pr.PRIVATE_MergeCommit.Oid != "" {
		converted.MergeCommit = &pr.PRIVATE_MergeCommit.Oid
	}

	return converted
}