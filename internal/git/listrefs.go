package git

import (
	"emperror.dev/errors"
	"strings"
)

type ListRefs struct {
	Patterns []string
}

type RefInfo struct {
	Name string
	Type string
	Oid  string
	// The name of the upstream ref (e.g., refs/remotes/<remote>/<branch>)
	Upstream string
	// The status of the ref relative to the upstream.
	UpstreamStatus UpstreamStatus
}

// ListRefs lists all refs in the repository (optionally matching a specific
// pattern).
func (r *Repo) ListRefs(showRef *ListRefs) ([]RefInfo, error) {
	// We want a subset of information about each ref, so we can tell Git to
	// print only specific fields. %00 is a null byte, which is the delimiter
	// that we split on while parsing the output.
	// See https://git-scm.com/docs/git-for-each-ref#_field_names for which
	// fields are available.
	const refInfoPattern = "%(refname)%00" + "%(objecttype)%00" +
		"%(objectname)%00" + "%(upstream)%00" + "%(upstream:track)"

	args := []string{"for-each-ref", "--format", refInfoPattern}
	if len(showRef.Patterns) > 0 {
		args = append(args, showRef.Patterns...)
	}
	out, err := r.Git(args...)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(out, "\n")
	refs := make([]RefInfo, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\x00")
		if len(parts) != 5 {
			return nil, errors.New("internal error: failed to parse ref info (expected 5 parts)")
		}
		refs = append(refs, RefInfo{
			Name:           parts[0],
			Type:           parts[1],
			Oid:            parts[2],
			Upstream:       parts[3],
			UpstreamStatus: UpstreamStatus(parts[4]),
		})
	}
	return refs, nil
}
