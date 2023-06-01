package gql

import (
	"context"

	"github.com/aviator-co/av/internal/config"
	"github.com/shurcooL/graphql"
	"golang.org/x/oauth2"
)

func GraphQLClient() *graphql.Client {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.Av.Aviator.APIToken},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	return graphql.NewClient("https://api.aviator.co/graphql", httpClient)
}
