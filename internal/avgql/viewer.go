package avgql

import (
	"emperror.dev/errors"
	"github.com/shurcooL/graphql"
)

// ViewerSubquery is a GraphQL query that fetches the viewer's email and full name.
// It's meant to be embedded into other queries to check if the user is authenticated
// via the CheckViewer method.
type ViewerSubquery struct {
	Viewer struct {
		Email    graphql.String `graphql:"email"`
		FullName graphql.String `graphql:"fullName"`
	}
}

var ErrNotAuthenticated = errors.New(
	"You are not logged in to Aviator. Please verify that your API token is correct.",
)

// CheckViewer checks whether or not the viewer is authenticated.
// It returns ErrNotAuthenticated if the viewer is not authenticated.
func (v ViewerSubquery) CheckViewer() error {
	if v.Viewer.Email == "" {
		return ErrNotAuthenticated
	}
	return nil
}
