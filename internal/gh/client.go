package gh

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/utils/logutils"
	"github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

type Client struct {
	httpClient *http.Client
	gh         *githubv4.Client
}

const githubApiBaseUrl = "https://api.github.com"

var once sync.Once
var client *Client // lazy init

func NewClient(token string) (*Client, error) {
	if token == "" {
		return nil, errors.Errorf("no GitHub token provided (do you need to configure one?)")
	}
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	return &Client{httpClient, githubv4.NewClient(httpClient)}, nil
}

func GetClient(token string) (*Client, error) {
	var err error
	once.Do(func() {
		client, err = NewClient(token)
	})
	return client, err
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

// restPost executes a POST request to the endpoint (e.g., /repos/:owner/:repo/pulls).
// It unmarshals the response into the given result type (unless it's nil).
func (c *Client) restPost(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
	if endpoint[0] != '/' {
		logrus.WithField("endpoint", endpoint).Panicf("malformed REST endpoint")
	}

	startTime := time.Now()
	url := githubApiBaseUrl + endpoint
	log := logrus.WithFields(logrus.Fields{
		"url":  url,
		"body": logutils.Format("%#+v", body),
	})
	bodyJson, err := json.Marshal(body)
	if err != nil {
		return errors.Wrap(err, "failed to marshal request body to JSON")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyJson))
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	log.Debug("executing GitHub API request...")
	res, err := c.httpClient.Do(req)
	log.Debugf("header: %#+v", req.Header)
	if err != nil {
		return errors.Wrap(err, "failed to make API request")
	}
	defer res.Body.Close()

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response body")
	}
	log.WithField("elapsed", time.Since(startTime)).Debug("GitHub API request completed")

	if res.StatusCode != http.StatusOK {
		log.WithFields(logrus.Fields{
			"status": res.StatusCode,
			"body":   string(resBody),
		}).Debug("GitHub API request failed")
		return errors.Errorf("GitHub API request for %s failed: %s", endpoint, res.Status)
	}

	// Don't try to unmarshal into nil, it will return an error.
	// NOTE: Go is weird with nil ("nil" can be typed or untyped) and this will
	// only capture an untyped nil (i.e., where the result parameter is given as
	// a nil literal), but that should be fine.
	if result == nil {
		return nil
	}

	if err := json.Unmarshal(resBody, result); err != nil {
		return errors.Wrap(err, "failed to unmarshal response body")
	}
	return nil
}
