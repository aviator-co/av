package git

import (
	"bytes"
	"emperror.dev/errors"
	"strings"
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
	// Need --quiet to suppress the diff that would otherwise be printed at the
	// end
	res, err := r.Git("show", "--quiet", "--format=%H%n%s%n%b", opts.Rev)
	if err != nil {
		return nil, err
	}
	var info CommitInfo
	buf := bytes.NewBufferString(res)
	info.Hash, err = readLine(buf)
	if err != nil {
		return nil, errors.Wrap(err, "git show: failed to parse commit hash")
	}
	info.Subject, err = readLine(buf)
	if err != nil {
		return nil, errors.Wrap(err, "git show: failed to parse commit subject")
	}
	info.Body, _ = buf.ReadString('\000')
	return &info, nil
}

func readLine(buf *bytes.Buffer) (string, error) {
	line, err := buf.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(line, "\n"), nil
}
