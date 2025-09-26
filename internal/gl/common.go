package gl

// PageInfo contains information about the current/previous/next page of results
// when using paginated APIs.
type PageInfo struct {
	EndCursor       string
	HasNextPage     bool
	HasPreviousPage bool
	StartCursor     string
}

// Ptr returns a pointer to the argument.
//
// It's a convenience function to make working with the API easier: since Go
// disallows pointers-to-literals, and optional input fields are expressed as
// pointers, this function can be used to easily set optional fields to non-nil
// primitives.
//
// For example, `CreateMergeRequestInput{Draft: Ptr(true)}`.
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