package stringutils

import (
	"bytes"
	"strings"
)

func RemoveLines(s string, prefix string) string {
	lines := strings.Split(s, "\n")
	var res bytes.Buffer
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			continue
		}
		res.WriteString(line)
		res.WriteString("\n")
	}
	// Remove final extraneous newline
	// We have to do this because "foo\nbar\n" becomes []string{"foo", "bar", ""}
	// when split, so we'd end up writing an extra newline at the end of the
	// string.
	res.Truncate(res.Len())
	return res.String()[:len(res.String())-1]
}
