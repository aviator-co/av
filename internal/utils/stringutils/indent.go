package stringutils

import "strings"

func Indent(s string, prefix string) string {
	// why is this not in the stdlib????
	return prefix + strings.Replace(s, "\n", "\n"+prefix, -1)
}
