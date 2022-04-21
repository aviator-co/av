package git

import (
	"emperror.dev/errors"
	"strings"
)

func (r *Repo) ListRefs(showRef *ListRefs) ([]RefInfo, error) {
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
			UpstreamStatus: parts[4],
		})
	}
	return refs, nil
}

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
