package git

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

var closeCommitPattern = regexp.MustCompile(
	`(?i)\b(?:close|closes|closed|fix|fixes|fixed|resolve|resolves|resolved)\W+#(\d+)\b`,
)

type LogOpts struct {
	// RevisionRange is the range of the commits specified by the format described in
	// git-log(1).
	RevisionRange []string
}

// Log returns a list of commits specified by the range.
func (r *Repo) Log(ctx context.Context, opts LogOpts) ([]*CommitInfo, error) {
	args := []string{"log", "--format=%H%x00%h%x00%s%x00%b%x00"}
	args = append(args, opts.RevisionRange...)
	args = append(args, "--")
	res, err := r.Run(ctx, &RunOpts{
		Args:      args,
		ExitError: true,
	})
	if err != nil {
		return nil, err
	}
	logrus.WithFields(logrus.Fields{"range": opts.RevisionRange}).Debug("got git-log")

	rd := bufio.NewReader(bytes.NewBuffer(res.Stdout))
	var ret []*CommitInfo
	for {
		ci, err := readLogEntry(rd)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		ret = append(ret, ci)
	}
	return ret, nil
}

func readLogEntry(rd *bufio.Reader) (*CommitInfo, error) {
	commitHash, err := rd.ReadString('\x00')
	if err != nil {
		return nil, err
	}
	abbrevHash, err := rd.ReadString('\x00')
	if err != nil {
		return nil, err
	}
	subject, err := rd.ReadString('\x00')
	if err != nil {
		return nil, err
	}
	body, err := rd.ReadString('\x00')
	if err != nil {
		return nil, err
	}
	return &CommitInfo{
		Hash:      strings.TrimSpace(trimNUL(commitHash)),
		ShortHash: strings.TrimSpace(trimNUL(abbrevHash)),
		Subject:   trimNUL(subject),
		Body:      trimNUL(body),
	}, nil
}

func trimNUL(s string) string {
	return strings.Trim(s, "\x00")
}

// FindClosesPullRequestComments finds the "closes #123" instructions from the commit messages. This
// returns a PR number to commit hash mapping.
//
// See https://docs.github.com/en/issues/tracking-your-work-with-issues/linking-a-pull-request-to-an-issue#linking-a-pull-request-to-an-issue-using-a-keyword
func FindClosesPullRequestComments(cis []*CommitInfo) map[int64]string {
	ret := map[int64]string{}
	for _, ci := range cis {
		matches := closeCommitPattern.FindAllStringSubmatch(ci.Body, -1)
		for _, match := range matches {
			prNum, _ := strconv.ParseInt(match[1], 10, 64)
			ret[prNum] = ci.Hash
		}
	}
	return ret
}
