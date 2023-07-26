package avgql

import (
	"context"
	"net/url"
	"os"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/config"
	"github.com/shurcooL/graphql"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

func NewClient() (*graphql.Client, error) {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.Av.Aviator.APIToken},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	apiURL := os.Getenv("AV_GRAPHQL_URL")
	if apiURL == "" {
		baseURL, err := url.Parse(config.Av.Aviator.APIHost)
		if err != nil {
			return nil, errors.WrapIff(err, "failed to parse aviator.apiHost configuration value")
		}
		apiURL = baseURL.JoinPath("/graphql").String()
	}
	logrus.WithField("api_url", apiURL).Debug("creating GraphQL client")
	return graphql.NewClient(apiURL, httpClient), nil
}
