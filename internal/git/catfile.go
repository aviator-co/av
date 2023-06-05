package git

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"strings"

	"emperror.dev/errors"
)

type GetRefs struct {
	// The revisions to retrieve.
	Revisions []string
}

type GetRefsItem struct {
	// The revision that was requested (exactly as given in GetRefs.Revisions)
	Revision string
	// The git object ID that the revision resolved to
	OID string
	// The type of the git object
	Type string
	// The contents of the git object
	Contents []byte
}

// GetRefs reads the contents of the specified objects from the repository.
// This corresponds to the `git cat-file --batch` command.
func (r *Repo) GetRefs(opts *GetRefs) ([]*GetRefsItem, error) {
	input := new(bytes.Buffer)
	for _, item := range opts.Revisions {
		input.WriteString(item)
		input.WriteString("\n")
	}

	res, err := r.Run(&RunOpts{
		Args:      []string{"cat-file", "--batch"},
		Stdin:     input,
		ExitError: true,
	})
	if err != nil {
		return nil, err
	}
	output := bytes.NewBuffer(res.Stdout)
	items := make([]*GetRefsItem, 0, len(opts.Revisions))
	for _, rev := range opts.Revisions {
		// The output *usually* looks like:
		//        <oid> SP <type> SP <size> LF
		//        <contents> LF
		// but it can also be
		//        <oid> SP {missing|ambiguous} LF

		item := GetRefsItem{Revision: rev}
		items = append(items, &item)
		_, err := fmt.Fscanf(output, "%s %s", &item.OID, &item.Type)
		if err != nil {
			return nil, errors.WrapIff(err, "failed to read cat-file output for revision %q", rev)
		}

		// special case where the rest of the format is ignored
		if item.Type == "missing" || item.Type == "ambiguous" {
			_, err := fmt.Fscanf(output, "\n")
			if err != nil {
				return nil, errors.Wrap(err, "failed to read cat-file output")
			}
			continue
		}

		// normal case: read the contents of the object
		var size int64
		_, err = fmt.Fscanf(output, "%d\n", &size)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read cat-file output")
		}
		item.Contents = make([]byte, size)
		if n, err := io.ReadFull(output, item.Contents); err != nil {
			logrus.WithFields(logrus.Fields{
				logrus.ErrorKey: err,
				"revision":      rev,
				"size":          size,
				"read":          n,
			}).Debug("failed to read contents from cat-file output")
			return nil, errors.WrapIff(err, "failed to read cat-file output for revision %q", rev)
		}

		// output includes a newline after the item contents
		// we have to read it otherwise subsequent iterations will fail
		// (Scanf refuses to consume the newline)
		lf, err := output.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				continue
			}
			return nil, errors.Wrap(err, "failed to read cat-file output")
		}
		if lf != '\n' {
			return nil, errors.New("failed to read cat-file output")
		}
	}

	return items, nil
}

type Commit struct {
	Tree      string
	Parents   []string
	Author    string
	Committer string
	Message   string
}

func (c *Commit) MessageTitle() string {
	if i := strings.Index(c.Message, "\n"); i >= 0 {
		return c.Message[:i]
	}
	return c.Message
}

func ParseCommitContents(contents []byte) (Commit, error) {
	// read line by line
	s := bufio.NewReader(bytes.NewReader(contents))
	var commit Commit
	for {
		line, err := s.ReadString('\n')
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return commit, err
		}

		if line == "\n" {
			// Blank line indicates the end of the header and the start of the
			// commit body.
			break
		}

		meta, value, ok := strings.Cut(line, " ")
		if !ok {
			return commit, errors.New("invalid commit format")
		}
		switch meta {
		case "tree":
			commit.Tree = value
		case "parent":
			commit.Parents = append(commit.Parents, value)
		case "author":
			commit.Author = value
		case "committer":
			commit.Committer = value
		default:
			// pass
		}
	}
	if commit.Tree == "" {
		return commit, errors.New("invalid commit format: missing tree")
	}
	if commit.Author == "" {
		return commit, errors.New("invalid commit format: missing author")
	}
	if commit.Committer == "" {
		return commit, errors.New("invalid commit format: missing committer")
	}

	// The rest of the commit contents is the commit message.
	// We don't need to parse it, so just read it as-is.
	message, err := io.ReadAll(s)
	if err != nil {
		return commit, err
	}
	commit.Message = string(message)

	return commit, nil
}
