package git

import (
	"context"
	"strings"

	"emperror.dev/errors"
)

// BlameLine represents a single line in a file as attributed by git blame.
type BlameLine struct {
	// CommitHash is the hash of the commit that introduced this line.
	CommitHash string
	// LineNo is the 1-based line number in the file.
	LineNo int
}

// Blame runs `git blame --porcelain` on the given file at the given revision
// and returns one BlameLine per line in the file.
//
// If the file does not exist at the given revision (e.g., a newly added file),
// an empty slice is returned with no error.
func (r *Repo) Blame(ctx context.Context, file string, rev string) ([]BlameLine, error) {
	out, err := r.Run(ctx, &RunOpts{
		Args: []string{"blame", "--porcelain", rev, "--", file},
	})
	if err != nil {
		// Non-nil error means git could not be started (not an exit code issue).
		return nil, errors.WrapIff(err, "git blame %s %s", rev, file)
	}
	if out.ExitCode != 0 {
		// git blame exits non-zero when the file does not exist at the given
		// revision (e.g., a newly added file being blamed at its parent commit).
		stderr := string(out.Stderr)
		if strings.Contains(stderr, "no such path") ||
			strings.Contains(stderr, "does not exist") ||
			strings.Contains(stderr, "fatal: no such") {
			return []BlameLine{}, nil
		}
		return nil, errors.Errorf("git blame %s %s: %s", rev, file, strings.TrimSpace(stderr))
	}

	return parsePorcelainBlame(out.Stdout)
}

// parsePorcelainBlame parses the output of `git blame --porcelain`.
//
// The porcelain format groups lines by commit. Each group starts with a header
// line of the form:
//
//	<hash> <orig-line> <final-line> [<num-lines>]
//
// Subsequent lines in the group are metadata lines (key-value pairs) until a
// line starting with a tab character, which is the actual source line content.
// The tab-prefixed line marks the end of the group for that source line.
func parsePorcelainBlame(data []byte) ([]BlameLine, error) {
	lines := strings.Split(string(data), "\n")

	var result []BlameLine
	var currentHash string
	var currentFinalLine int
	inHeader := false

	for _, line := range lines {
		if line == "" {
			continue
		}

		if line[0] == '\t' {
			// This is the source line content — it marks the end of a blame group.
			// We've already captured the hash and line number from the header.
			if currentHash != "" {
				result = append(result, BlameLine{
					CommitHash: currentHash,
					LineNo:     currentFinalLine,
				})
			}
			inHeader = false
			currentHash = ""
			currentFinalLine = 0
			continue
		}

		if !inHeader {
			// A new blame group header: "<hash> <orig-line> <final-line> [<count>]"
			fields := strings.Fields(line)
			if len(fields) < 3 {
				return nil, errors.Errorf("git blame: unexpected header line: %q", line)
			}
			// Validate that the first field looks like a commit hash (40 hex chars).
			if len(fields[0]) != 40 {
				return nil, errors.Errorf("git blame: unexpected hash %q in line: %q", fields[0], line)
			}

			finalLine := 0
			_, err := parseIntField(fields[2], &finalLine)
			if err != nil {
				return nil, errors.Errorf("git blame: failed to parse final line number from %q: %v", line, err)
			}

			currentHash = fields[0]
			currentFinalLine = finalLine
			inHeader = true
		}
		// Other lines in the header block are metadata (author, committer, etc.) — skip them.
	}

	return result, nil
}

// parseIntField parses an integer from a string field into the destination pointer.
func parseIntField(s string, dest *int) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.Errorf("not a valid integer: %q", s)
		}
		n = n*10 + int(c-'0')
	}
	*dest = n
	return n, nil
}
