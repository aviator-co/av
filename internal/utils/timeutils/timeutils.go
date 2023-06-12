package timeutils

import (
	"time"
)

// FormalLocal takes a time and converts it into a readable format in the local timezome (outputLayout).
func FormatLocal(timestamp time.Time) string {
	outputLayout := "2 January 2006 3:04:05 PM PST"
	timestamp = timestamp.In(time.Local)
	return timestamp.Format(outputLayout)
}
