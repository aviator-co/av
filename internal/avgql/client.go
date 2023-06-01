package avgql

import (
	"context"
	"fmt"

	"github.com/aviator-co/av/internal/config"
	"github.com/shurcooL/graphql"
	"golang.org/x/oauth2"
)

func NewClient() *graphql.Client {
	fmt.Println("token: ", config.Av.Aviator.APIToken)
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.Av.Aviator.APIToken},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	return graphql.NewClient("https://api.aviator.co/graphql", httpClient)
}
