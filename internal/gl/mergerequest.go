package gl

import (
	"context"
	"time"

	"emperror.dev/errors"
)

type MergeRequest struct {
	ID          GitLabID          `graphql:"id"`
	IID         int64             `graphql:"iid"` // Internal ID used in URLs
	Title       string            `graphql:"title"`
	Description string            `graphql:"description"`
	State       MergeRequestState `graphql:"state"`
	Draft       bool              `graphql:"draft"`
	WebURL      string            `graphql:"webUrl"`
	CreatedAt   GitLabTime        `graphql:"createdAt"`
	UpdatedAt   GitLabTime        `graphql:"updatedAt"`
	
	SourceBranch string `graphql:"sourceBranch"`
	TargetBranch string `graphql:"targetBranch"`
	
	MergeCommitSha *string `graphql:"mergeCommitSha"`
	
	// Author information
	Author struct {
		Username string `graphql:"username"`
		Name     string `graphql:"name"`
	} `graphql:"author"`
	
	// Project information  
	Project struct {
		ID       GitLabID `graphql:"id"`
		FullPath string   `graphql:"fullPath"`
	} `graphql:"project"`
}

// CreateMergeRequestInput represents the input for creating a merge request
type CreateMergeRequestInput struct {
	ProjectPath         string
	Title               string
	Description         string
	SourceBranch        string
	TargetBranch        string
	Draft               bool
	RemoveSourceBranch  bool
	Squash              bool
}

// UpdateMergeRequestInput represents the input for updating a merge request
type UpdateMergeRequestInput struct {
	ProjectPath  string
	IID          int64
	Title        *string
	Description  *string
	TargetBranch *string
	Draft        *bool
}

func (c *Client) GetMergeRequest(ctx context.Context, projectPath string, iid int64) (*MergeRequest, error) {
	var query struct {
		Project struct {
			MergeRequest MergeRequest `graphql:"mergeRequest(iid: $iid)"`
		} `graphql:"project(fullPath: $projectPath)"`
	}

	variables := map[string]any{
		"projectPath": projectPath,
		"iid":         iid,
	}

	if err := c.query(ctx, &query, variables); err != nil {
		return nil, WrapError(err, "get merge request")
	}

	if query.Project.MergeRequest.ID == "" {
		return nil, errors.Errorf("merge request not found: %s!%d", projectPath, iid)
	}

	return &query.Project.MergeRequest, nil
}

func (c *Client) GetMergeRequests(ctx context.Context, projectPath string, state *MergeRequestState, sourceBranch *string, targetBranch *string) ([]*MergeRequest, error) {
	var query struct {
		Project struct {
			MergeRequests struct {
				Nodes []MergeRequest `graphql:"nodes"`
			} `graphql:"mergeRequests(first: 100, state: $state, sourceBranches: $sourceBranches, targetBranches: $targetBranches)"`
		} `graphql:"project(fullPath: $projectPath)"`
	}

	variables := map[string]any{
		"projectPath": projectPath,
	}

	if state != nil {
		variables["state"] = *state
	}
	if sourceBranch != nil {
		variables["sourceBranches"] = []string{*sourceBranch}
	}
	if targetBranch != nil {
		variables["targetBranches"] = []string{*targetBranch}
	}

	if err := c.query(ctx, &query, variables); err != nil {
		return nil, WrapError(err, "get merge requests")
	}

	result := make([]*MergeRequest, len(query.Project.MergeRequests.Nodes))
	for i, mr := range query.Project.MergeRequests.Nodes {
		result[i] = &mr
	}

	return result, nil
}

func (c *Client) CreateMergeRequest(ctx context.Context, input *CreateMergeRequestInput) (*MergeRequest, error) {
	var mutation struct {
		MergeRequestCreate struct {
			MergeRequest MergeRequest `graphql:"mergeRequest"`
			Errors       []string     `graphql:"errors"`
		} `graphql:"mergeRequestCreate(input: $input)"`
	}

	variables := map[string]any{
		"input": map[string]any{
			"projectPath":        input.ProjectPath,
			"title":              input.Title,
			"description":        input.Description,
			"sourceBranch":       input.SourceBranch,
			"targetBranch":       input.TargetBranch,
			"draft":              input.Draft,
			"removeSourceBranch": input.RemoveSourceBranch,
			"squash":             input.Squash,
		},
	}

	if err := c.mutate(ctx, &mutation, variables); err != nil {
		return nil, WrapError(err, "create merge request")
	}

	if len(mutation.MergeRequestCreate.Errors) > 0 {
		return nil, errors.Errorf("failed to create merge request: %v", mutation.MergeRequestCreate.Errors)
	}

	return &mutation.MergeRequestCreate.MergeRequest, nil
}

func (c *Client) UpdateMergeRequest(ctx context.Context, input *UpdateMergeRequestInput) (*MergeRequest, error) {
	var mutation struct {
		MergeRequestUpdate struct {
			MergeRequest MergeRequest `graphql:"mergeRequest"`
			Errors       []string     `graphql:"errors"`
		} `graphql:"mergeRequestUpdate(input: $input)"`
	}

	mutationInput := map[string]any{
		"projectPath": input.ProjectPath,
		"iid":         input.IID,
	}

	if input.Title != nil {
		mutationInput["title"] = *input.Title
	}
	if input.Description != nil {
		mutationInput["description"] = *input.Description
	}
	if input.TargetBranch != nil {
		mutationInput["targetBranch"] = *input.TargetBranch
	}
	if input.Draft != nil {
		mutationInput["draft"] = *input.Draft
	}

	variables := map[string]any{
		"input": mutationInput,
	}

	if err := c.mutate(ctx, &mutation, variables); err != nil {
		return nil, WrapError(err, "update merge request")
	}

	if len(mutation.MergeRequestUpdate.Errors) > 0 {
		return nil, errors.Errorf("failed to update merge request: %v", mutation.MergeRequestUpdate.Errors)
	}

	return &mutation.MergeRequestUpdate.MergeRequest, nil
}

func (c *Client) ConvertMergeRequestToDraft(ctx context.Context, projectPath string, iid int64) (*MergeRequest, error) {
	return c.UpdateMergeRequest(ctx, &UpdateMergeRequestInput{
		ProjectPath: projectPath,
		IID:         iid,
		Draft:       &[]bool{true}[0],
	})
}

func (c *Client) MarkMergeRequestReadyForReview(ctx context.Context, projectPath string, iid int64) (*MergeRequest, error) {
	return c.UpdateMergeRequest(ctx, &UpdateMergeRequestInput{
		ProjectPath: projectPath,
		IID:         iid,
		Draft:       &[]bool{false}[0],
	})
}