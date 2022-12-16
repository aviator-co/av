package stringutils

import "strings"

// ParseSubjectBody parses the subject and body from a multiline string.
// The subject is the first line of the string, and the body is the rest.
// Newlines surrounding the body are trimmed.
func ParseSubjectBody(s string) (subject string, body string) {
	subject, body, _ = strings.Cut(strings.Trim(s, "\n"), "\n")
	return strings.Trim(subject, "\n"), strings.Trim(body, "\n")
}
