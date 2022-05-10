package gh

import (
	"context"
	"github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"time"
)

type Client struct {
	gh *githubv4.Client
}

func NewClient(token string) (*Client, error) {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	return &Client{githubv4.NewClient(httpClient)}, nil
}

func (c *Client) query(ctx context.Context, query any, variables map[string]any) (reterr error) {
	startTime := time.Now()
	defer func() {
		log := logrus.WithField("time", time.Since(startTime))
		if reterr != nil {
			log.WithError(reterr).Debug("GitHub API query failed")
		} else {
			log.Debug("GitHub API query succeeded")
		}
	}()
	return c.gh.Query(ctx, query, variables)
}

func (c *Client) mutate(ctx context.Context, mutation any, input githubv4.Input, variables map[string]any) (reterr error) {
	startTime := time.Now()
	defer func() {
		log := logrus.WithField("time", time.Since(startTime))
		if reterr != nil {
			log.WithError(reterr).Debug("GitHub API mutation failed")
		} else {
			log.Debug("GitHub API mutation succeeded")
		}
	}()
	return c.gh.Mutate(ctx, mutation, input, variables)
}

// Ptr returns a pointer to the argument.
// It's a convenience function to make working with the API easier: since Go
// disallows pointers-to-literals, and optional input fields are expressed as
// pointers, this function can be used to easily set optional fields to non-nil
// primitives.
// For example, githubv4.CreatePullRequestInput{Draft: Ptr(true)}
func Ptr[T any](v T) *T {
	return &v
}
