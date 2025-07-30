package gl

import (
	"context"
	"net/http"
	"time"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/utils/logutils"
	"github.com/shurcooL/graphql"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

type Client struct {
	httpClient *http.Client
	gl         *graphql.Client
}

// NewClient creates a new GitLab client.
// It takes configuration from the global config.Av.GitLab variable.
func NewClient(ctx context.Context, token string) (*Client, error) {
	if token == "" {
		return nil, errors.Errorf("no GitLab token provided (do you need to configure one?)")
	}
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	httpClient := oauth2.NewClient(ctx, src)
	
	var graphqlURL string
	if config.Av.GitLab.BaseURL == "" {
		graphqlURL = "https://gitlab.com/api/graphql"
	} else {
		graphqlURL = config.Av.GitLab.BaseURL + "/api/graphql"
	}
	
	gl := graphql.NewClient(graphqlURL, httpClient)
	return &Client{httpClient, gl}, nil
}

func (c *Client) query(ctx context.Context, query any, variables map[string]any) (reterr error) {
	log := logrus.WithFields(logrus.Fields{
		"variables": logutils.Format("%#+v", variables),
	})
	log.Debug("executing GitLab API query...")
	startTime := time.Now()
	defer func() {
		log := log.WithFields(logrus.Fields{
			"elapsed": time.Since(startTime),
			"result":  logutils.Format("%#+v", query),
		})
		if reterr != nil {
			log.WithError(reterr).Debug("GitLab API query failed")
		} else {
			log.Debug("GitLab API query succeeded")
		}
	}()
	return c.gl.Query(ctx, query, variables)
}

func (c *Client) mutate(ctx context.Context, mutation any, variables map[string]any) (reterr error) {
	log := logrus.WithFields(logrus.Fields{
		"variables": logutils.Format("%#+v", variables),
	})
	log.Debug("executing GitLab API mutation...")
	startTime := time.Now()
	defer func() {
		log := log.WithFields(logrus.Fields{
			"elapsed": time.Since(startTime),
			"result":  logutils.Format("%#+v", mutation),
		})
		if reterr != nil {
			log.WithError(reterr).Debug("GitLab API mutation failed")
		} else {
			log.Debug("GitLab API mutation succeeded")
		}
	}()
	return c.gl.Mutate(ctx, mutation, variables)
}