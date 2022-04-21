package git

// Missing is a sentinel zero-value for object id (aka sha).
// Git treats this value as "this thing doesn't exist".
// For example, when updating a ref, if the old value is specified as EmptyOid,
// Git will refuse to update the ref if already exists.
const Missing = "0000000000000000000000000000000000000000"

// The various types of objects in git.
const (
	TypeCommit = "commit"
	TypeTree   = "tree"
	TypeBlob   = "blob"
	TypeTag    = "tag"
)

// UpstreamStatus is the status of a git ref (usually a branch) relative to its
// upstream.
type UpstreamStatus = string

// The possible upstream statuses.
// These match what is returned by Git's `%(upstream:trackshort)` format directive.
const (
	Ahead     UpstreamStatus = ">"
	Behind    UpstreamStatus = "<"
	Divergent UpstreamStatus = "<>"
	InSync    UpstreamStatus = "="
)
