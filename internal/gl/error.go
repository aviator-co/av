package gl

import (
	"fmt"
	"strings"

	"emperror.dev/errors"
	"github.com/shurcooL/graphql"
)

// GitLabError wraps GitLab API errors with additional context.
type GitLabError struct {
	Message string
	Type    string
	Code    string
}

func (e *GitLabError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("GitLab API error (%s): %s", e.Code, e.Message)
	}
	return fmt.Sprintf("GitLab API error: %s", e.Message)
}

// WrapError wraps a GitLab GraphQL error with additional context.
func WrapError(err error, operation string) error {
	if err == nil {
		return nil
	}

	// Handle GraphQL errors
	var gqlErrors graphql.Errors
	if errors.As(err, &gqlErrors) {
		if len(gqlErrors) > 0 {
			// Take the first error and wrap it
			gqlErr := gqlErrors[0]
			glErr := &GitLabError{
				Message: gqlErr.Message,
				Type:    gqlErr.Type,
			}
			
			// Extract error code if available in extensions
			if gqlErr.Extensions != nil {
				if code, ok := gqlErr.Extensions["code"].(string); ok {
					glErr.Code = code
				}
			}
			
			return errors.Wrapf(glErr, "GitLab %s failed", operation)
		}
	}

	// Handle common GitLab API error patterns
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "401"):
		return errors.Wrapf(err, "GitLab %s failed: authentication required (check your token)", operation)
	case strings.Contains(errMsg, "403"):
		return errors.Wrapf(err, "GitLab %s failed: insufficient permissions", operation)
	case strings.Contains(errMsg, "404"):
		return errors.Wrapf(err, "GitLab %s failed: resource not found", operation)
	case strings.Contains(errMsg, "422"):
		return errors.Wrapf(err, "GitLab %s failed: invalid request data", operation)
	case strings.Contains(errMsg, "429"):
		return errors.Wrapf(err, "GitLab %s failed: rate limit exceeded", operation)
	case strings.Contains(errMsg, "500"), strings.Contains(errMsg, "502"), strings.Contains(errMsg, "503"):
		return errors.Wrapf(err, "GitLab %s failed: server error", operation)
	default:
		return errors.Wrapf(err, "GitLab %s failed", operation)
	}
}