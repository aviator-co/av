package sanitize

import (
	"regexp"
	"strings"
)

const (
	fileNameMax = 100
)

var filenameReplacePattern = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func FileName(name string) string {
	name = strings.ToLower(name)
	name = filenameReplacePattern.ReplaceAllString(name, "-")
	if len(name) > fileNameMax {
		name = name[:fileNameMax]
	}
	name = strings.Trim(name, "-")
	return name
}
