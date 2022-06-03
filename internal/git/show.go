package git

import (
	"bytes"
	"emperror.dev/errors"
	"fmt"
)

type CommitInfoOpts struct {
	Rev string
}

type CommitInfo struct {
	Hash    string
	Subject string
	Body    string
}

func (r *Repo) CommitInfo(opts CommitInfoOpts) (*CommitInfo, error) {
	res, err := r.Git("show", "--format=%H%n%s%n%b", opts.Rev)
	if err != nil {
		return nil, err
	}
	var info CommitInfo
	buf := bytes.NewBufferString(res)
	if _, err := fmt.Fscanf(buf, "%s\n%s\n", &info.Hash, &info.Subject); err != nil {
		return nil, errors.WrapIff(err, "failed to read output of git show")
	}
	// Note: buf.ReadString returns error iff buffer doesn't contain the delimiter,
	// which is expected, so we just ignore the error.
	info.Body, _ = buf.ReadString('\000')
	return &info, nil
}
