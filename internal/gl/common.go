package gl

import "time"

// MergeRequestState represents the state of a GitLab merge request.
type MergeRequestState string

const (
	MergeRequestStateOpened MergeRequestState = "opened"
	MergeRequestStateClosed MergeRequestState = "closed"
	MergeRequestStateMerged MergeRequestState = "merged"
)

// MergeStatus represents the merge status of a GitLab merge request.
type MergeStatus string

const (
	MergeStatusCanBeMerged       MergeStatus = "can_be_merged"
	MergeStatusCannotBeMerged    MergeStatus = "cannot_be_merged" 
	MergeStatusUnchecked         MergeStatus = "unchecked"
	MergeStatusCannotBeMergedRecheck MergeStatus = "cannot_be_merged_recheck"
)

// Common GraphQL fragment structures used across GitLab API calls
type PageInfo struct {
	HasNextPage     bool   `graphql:"hasNextPage"`
	HasPreviousPage bool   `graphql:"hasPreviousPage"`
	StartCursor     string `graphql:"startCursor"`
	EndCursor       string `graphql:"endCursor"`
}

// GitLab uses different field names than GitHub
type GitLabID string

func (id GitLabID) String() string {
	return string(id)
}

// Common time handling for GitLab API responses
type GitLabTime struct {
	time.Time
}

func (t *GitLabTime) UnmarshalJSON(data []byte) error {
	// GitLab uses ISO 8601 format: "2023-01-01T00:00:00.000Z"
	str := string(data)
	if str == "null" {
		return nil
	}
	// Remove quotes
	str = str[1 : len(str)-1]
	
	parsed, err := time.Parse("2006-01-02T15:04:05.000Z", str)
	if err != nil {
		// Try alternative format without milliseconds
		parsed, err = time.Parse("2006-01-02T15:04:05Z", str)
		if err != nil {
			return err
		}
	}
	t.Time = parsed
	return nil
}