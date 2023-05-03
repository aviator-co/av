package git

import (
	"bytes"
	"strings"

	"emperror.dev/errors"
	"github.com/sirupsen/logrus"
)

type CommitInfoOpts struct {
	Rev string
}

type CommitInfo struct {
	Hash      string
	ShortHash string
	Subject   string
	Body      string
}

func (c CommitInfo) BodyWithPrefix(prefix string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(c.Body), "\n") {
		lines = append(lines, prefix+line)
	}
	return lines
}

func (r *Repo) CommitInfo(opts CommitInfoOpts) (*CommitInfo, error) {
	// Need --quiet to suppress the diff that would otherwise be printed at the
	// end
	res, err := r.Run(&RunOpts{
		Args:      []string{"show", "--quiet", "--format=%H%n%s%n%b", opts.Rev},
		ExitError: true,
	})
	if err != nil {
		return nil, err
	}
	logrus.WithFields(logrus.Fields{"output": string(res.Stdout), "rev": opts.Rev}).Debug("got commit info")
	var info CommitInfo
	buf := bytes.NewBuffer(res.Stdout)
	info.Hash, err = readLine(buf)
	if err != nil {
		return nil, errors.Wrap(err, "git show: failed to parse commit hash")
	}
	info.ShortHash = ShortSha(info.Hash)
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
