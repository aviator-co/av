package avgql

import (
	"context"
	"net/http"
	"net/url"
	"os"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/config"
	"github.com/shurcooL/graphql"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

func NewClient(ctx context.Context) (*graphql.Client, error) {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.Av.Aviator.APIToken},
	)
	httpClient := oauth2.NewClient(ctx, src)
	httpClient.Transport = clientNameTransport{base: httpClient.Transport}
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

// clientNameTransport identifies av to the Aviator API, which counts requests per client.
type clientNameTransport struct {
	base http.RoundTripper
}

func (t clientNameTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Apollographql-Client-Name", "av CLI")
	req.Header.Set("Apollographql-Client-Version", config.Version)
	return t.base.RoundTrip(req)
}
