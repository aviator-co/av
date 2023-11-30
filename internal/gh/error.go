package gh

import "strings"

// IsHTTPUnauthorized returns true if the given error is an HTTP 401 Unauthorized error.
func IsHTTPUnauthorized(err error) bool {
	// This is a bit fragile because it relies on the error message from the
	// GraphQL package. It doesn't export proper error types so we have to check
	// the string.
	return strings.Contains(err.Error(), "status code: 401")
}
