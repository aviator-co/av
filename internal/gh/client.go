package gh

import (
	"context"
	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/utils/logutils"
	"github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"time"
)

type Client struct {
	gh *githubv4.Client
}

func NewClient(token string) (*Client, error) {
	if token == "" {
		return nil, errors.Errorf("no GitHub token provided (do you need to configure one?)")
	}
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	return &Client{githubv4.NewClient(httpClient)}, nil
}

func (c *Client) query(ctx context.Context, query any, variables map[string]any) (reterr error) {
	log := logrus.WithFields(logrus.Fields{
		"variables": logutils.Format("%#+v", variables),
	})
	log.Debug("executing GitHub API query...")
	startTime := time.Now()
	defer func() {
		log := log.WithFields(logrus.Fields{
			"elapsed": time.Since(startTime),
			"result":  logutils.Format("%#+v", query),
		})
		if reterr != nil {
			log.WithError(reterr).Debug("GitHub API query failed")
		} else {
			log.Debug("GitHub API query succeeded")
		}
	}()
	return c.gh.Query(ctx, query, variables)
}

func (c *Client) mutate(ctx context.Context, mutation any, input githubv4.Input, variables map[string]any) (reterr error) {
	log := logrus.WithFields(logrus.Fields{
		"input": logutils.Format("%#+v", input),
	})
	log.Debug("executing GitHub API mutation...")
	startTime := time.Now()
	defer func() {
		log := log.WithFields(logrus.Fields{
			"elapsed": time.Since(startTime),
			"result":  logutils.Format("%#+v", mutation),
		})
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

// nullable returns a pointer to the argument if it's not the zero value,
// otherwise it returns nil.
// This is useful to translate between Golang-style "unset is zero" and GraphQL
// which distinguishes between unset (null) and zero values.
func nullable[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}
