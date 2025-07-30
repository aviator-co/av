package providers

import (
	"context"
	"time"
)

// Client defines the interface for Git hosting provider clients.
// This interface abstracts operations across GitHub and GitLab.
type Client interface {
	// Repository operations
	GetRepository(ctx context.Context, slug string) (*Repository, error)

	// Pull/Merge Request operations
	GetPullRequest(ctx context.Context, id string) (*PullRequest, error)
	CreatePullRequest(ctx context.Context, opts *CreatePullRequestOpts) (*PullRequest, error)
	UpdatePullRequest(ctx context.Context, opts *UpdatePullRequestOpts) (*PullRequest, error)
	GetPullRequests(ctx context.Context, opts *GetPullRequestsOpts) ([]*PullRequest, error)

	// Draft operations
	ConvertToDraft(ctx context.Context, id string) (*PullRequest, error)
	MarkReadyForReview(ctx context.Context, id string) (*PullRequest, error)

	// Review operations
	RequestReviews(ctx context.Context, id string, reviewers []string, teamReviewers []string) error

	// User operations
	GetCurrentUser(ctx context.Context) (*User, error)
	GetUser(ctx context.Context, login string) (*User, error)
}

// Repository represents a Git repository.
type Repository struct {
	ID       string
	Owner    string
	Name     string
	FullName string // e.g., "owner/repo"
}

// PullRequestState represents the state of a pull/merge request.
type PullRequestState string

const (
	PullRequestStateOpen   PullRequestState = "open"
	PullRequestStateClosed PullRequestState = "closed" 
	PullRequestStateMerged PullRequestState = "merged"
)

// PullRequest represents a pull request (GitHub) or merge request (GitLab).
type PullRequest struct {
	ID          string
	Number      int64
	Title       string
	Body        string
	State       PullRequestState
	IsDraft     bool
	HeadRefName string // Source branch
	BaseRefName string // Target branch
	Permalink   string // Web URL
	MergeCommit *string // Commit SHA if merged, nil otherwise
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CreatePullRequestOpts contains options for creating a pull/merge request.
type CreatePullRequestOpts struct {
	Repository  string   // Repository slug (owner/repo)
	Title       string
	Body        string
	HeadRefName string   // Source branch
	BaseRefName string   // Target branch
	IsDraft     bool
	Reviewers   []string // Usernames to request reviews from
}

// UpdatePullRequestOpts contains options for updating a pull/merge request.
type UpdatePullRequestOpts struct {
	ID          string
	Title       *string // nil means don't update
	Body        *string // nil means don't update
	BaseRefName *string // nil means don't update
}

// GetPullRequestsOpts contains options for listing pull/merge requests.
type GetPullRequestsOpts struct {
	Repository string
	State      *PullRequestState // nil means all states
	HeadRefName *string          // Filter by source branch
	BaseRefName *string          // Filter by target branch
}

// User represents a user account.
type User struct {
	ID    string
	Login string // Username
	Name  string // Display name
	Email string
}