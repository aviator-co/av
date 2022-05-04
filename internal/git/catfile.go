package git

import (
	"bytes"
	"emperror.dev/errors"
	"fmt"
	"io"
)

type GetRefs struct {
	// The revisions to retrieve.
	Revisions []string
}

type GetRefsItem struct {
	// The revision that was requested (exactly as given in GetRefs.Revisions)
	Revision string
	// The git object ID that the revision resolved to
	Oid string
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

	res, err := r.GitStdin([]string{"cat-file", "--batch"}, input)
	if err != nil {
		return nil, err
	}
	output := bytes.NewBufferString(res)
	items := make([]*GetRefsItem, 0, len(opts.Revisions))
	for _, rev := range opts.Revisions {
		// The output *usually* looks like:
		//        <oid> SP <type> SP <size> LF
		//        <contents> LF
		// but it can also be
		//        <oid> SP {missing|ambiguous} LF

		item := GetRefsItem{Revision: rev}
		items = append(items, &item)
		_, err := fmt.Fscanf(output, "%s %s", &item.Oid, &item.Type)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read cat-file output")
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
		_, err = io.ReadFull(output, item.Contents)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read cat-file output")
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
