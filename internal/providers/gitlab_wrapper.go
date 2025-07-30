package providers

import (
	"context"
	"strconv"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/gl"
)

// GitLabClientWrapper wraps the GitLab client to implement our provider interface.
type GitLabClientWrapper struct {
	client   *gl.Client
	repoSlug string
}

func (w *GitLabClientWrapper) GetRepository(ctx context.Context, slug string) (*Repository, error) {
	repo, err := w.client.GetRepositoryBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	owner, repoName, ok := gl.ParseFullPath(repo.FullPath)
	if !ok {
		return nil, errors.Errorf("invalid repository path: %s", repo.FullPath)
	}

	return &Repository{
		ID:       repo.ID.String(),
		Owner:    owner,
		Name:     repoName,
		FullName: repo.FullPath,
	}, nil
}

func (w *GitLabClientWrapper) GetPullRequest(ctx context.Context, id string) (*PullRequest, error) {
	// For GitLab, we need to parse the ID to get project path and MR IID
	// Expected format: "project/path!123" or just "123" if using current repo
	var projectPath string
	var iid int64
	var err error

	if strings.Contains(id, "!") {
		parts := strings.Split(id, "!")
		if len(parts) != 2 {
			return nil, errors.Errorf("invalid GitLab merge request ID format: %s", id)
		}
		projectPath = parts[0]
		iid, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, errors.Errorf("invalid merge request IID: %s", parts[1])
		}
	} else {
		// Use current repository
		projectPath = w.repoSlug
		iid, err = strconv.ParseInt(id, 10, 64)
		if err != nil {
			return nil, errors.Errorf("invalid merge request IID: %s", id)
		}
	}

	mr, err := w.client.GetMergeRequest(ctx, projectPath, iid)
	if err != nil {
		return nil, err
	}

	return w.convertGitLabMR(mr), nil
}

func (w *GitLabClientWrapper) CreatePullRequest(ctx context.Context, opts *CreatePullRequestOpts) (*PullRequest, error) {
	input := &gl.CreateMergeRequestInput{
		ProjectPath:        opts.Repository,
		Title:              opts.Title,
		Description:        opts.Body,
		SourceBranch:       opts.HeadRefName,
		TargetBranch:       opts.BaseRefName,
		Draft:              opts.IsDraft,
		RemoveSourceBranch: false, // Keep source branch by default
		Squash:             false, // Don't squash by default
	}

	mr, err := w.client.CreateMergeRequest(ctx, input)
	if err != nil {
		return nil, err
	}

	// Request reviews if specified
	if len(opts.Reviewers) > 0 {
		if err := w.client.RequestReviews(ctx, opts.Repository, mr.IID, opts.Reviewers); err != nil {
			// Don't fail the entire operation if review requests fail
			// Just log and continue
		}
	}

	return w.convertGitLabMR(mr), nil
}

func (w *GitLabClientWrapper) UpdatePullRequest(ctx context.Context, opts *UpdatePullRequestOpts) (*PullRequest, error) {
	// Parse GitLab MR ID to get IID
	projectPath, iid, err := w.parseGitLabMRID(opts.ID)
	if err != nil {
		return nil, err
	}

	input := &gl.UpdateMergeRequestInput{
		ProjectPath: projectPath,
		IID:         iid,
		Title:       opts.Title,
		Description: opts.Body,
		TargetBranch: opts.BaseRefName,
	}

	mr, err := w.client.UpdateMergeRequest(ctx, input)
	if err != nil {
		return nil, err
	}

	return w.convertGitLabMR(mr), nil
}

func (w *GitLabClientWrapper) GetPullRequests(ctx context.Context, opts *GetPullRequestsOpts) ([]*PullRequest, error) {
	var state *gl.MergeRequestState
	if opts.State != nil {
		switch *opts.State {
		case PullRequestStateOpen:
			state = &[]gl.MergeRequestState{gl.MergeRequestStateOpened}[0]
		case PullRequestStateClosed:
			state = &[]gl.MergeRequestState{gl.MergeRequestStateClosed}[0]
		case PullRequestStateMerged:
			state = &[]gl.MergeRequestState{gl.MergeRequestStateMerged}[0]
		}
	}

	mrs, err := w.client.GetMergeRequests(ctx, opts.Repository, state, opts.HeadRefName, opts.BaseRefName)
	if err != nil {
		return nil, err
	}

	result := make([]*PullRequest, len(mrs))
	for i, mr := range mrs {
		result[i] = w.convertGitLabMR(mr)
	}

	return result, nil
}

func (w *GitLabClientWrapper) ConvertToDraft(ctx context.Context, id string) (*PullRequest, error) {
	projectPath, iid, err := w.parseGitLabMRID(id)
	if err != nil {
		return nil, err
	}

	mr, err := w.client.ConvertMergeRequestToDraft(ctx, projectPath, iid)
	if err != nil {
		return nil, err
	}

	return w.convertGitLabMR(mr), nil
}

func (w *GitLabClientWrapper) MarkReadyForReview(ctx context.Context, id string) (*PullRequest, error) {
	projectPath, iid, err := w.parseGitLabMRID(id)
	if err != nil {
		return nil, err
	}

	mr, err := w.client.MarkMergeRequestReadyForReview(ctx, projectPath, iid)
	if err != nil {
		return nil, err
	}

	return w.convertGitLabMR(mr), nil
}

func (w *GitLabClientWrapper) RequestReviews(ctx context.Context, id string, reviewers []string, teamReviewers []string) error {
	projectPath, iid, err := w.parseGitLabMRID(id)
	if err != nil {
		return err
	}

	// GitLab doesn't have team reviewers in the same way as GitHub
	// We'll just use individual reviewers
	return w.client.RequestReviews(ctx, projectPath, iid, reviewers)
}

func (w *GitLabClientWrapper) GetCurrentUser(ctx context.Context) (*User, error) {
	user, err := w.client.GetCurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	return &User{
		ID:    user.ID.String(),
		Login: user.Username,
		Name:  user.Name,
		Email: user.Email,
	}, nil
}

func (w *GitLabClientWrapper) GetUser(ctx context.Context, login string) (*User, error) {
	user, err := w.client.GetUser(ctx, login)
	if err != nil {
		return nil, err
	}

	return &User{
		ID:    user.ID.String(),
		Login: user.Username,
		Name:  user.Name,
		Email: user.Email,
	}, nil
}

func (w *GitLabClientWrapper) convertGitLabMR(mr *gl.MergeRequest) *PullRequest {
	var state PullRequestState
	switch mr.State {
	case gl.MergeRequestStateOpened:
		state = PullRequestStateOpen
	case gl.MergeRequestStateClosed:
		state = PullRequestStateClosed
	case gl.MergeRequestStateMerged:
		state = PullRequestStateMerged
	}

	converted := &PullRequest{
		ID:          mr.ID.String(),
		Number:      mr.IID,
		Title:       mr.Title,
		Body:        mr.Description,
		State:       state,
		IsDraft:     mr.Draft,
		HeadRefName: mr.SourceBranch,
		BaseRefName: mr.TargetBranch,
		Permalink:   mr.WebURL,
		CreatedAt:   mr.CreatedAt.Time,
		UpdatedAt:   mr.UpdatedAt.Time,
	}

	// Handle merge commit if available
	if mr.MergeCommitSha != nil {
		converted.MergeCommit = mr.MergeCommitSha
	}

	return converted
}

func (w *GitLabClientWrapper) parseGitLabMRID(id string) (projectPath string, iid int64, err error) {
	if strings.Contains(id, "!") {
		parts := strings.Split(id, "!")
		if len(parts) != 2 {
			return "", 0, errors.Errorf("invalid GitLab merge request ID format: %s", id)
		}
		projectPath = parts[0]
		iid, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return "", 0, errors.Errorf("invalid merge request IID: %s", parts[1])
		}
	} else {
		// Use current repository
		projectPath = w.repoSlug
		iid, err = strconv.ParseInt(id, 10, 64)
		if err != nil {
			return "", 0, errors.Errorf("invalid merge request IID: %s", id)
		}
	}
	return projectPath, iid, nil
}