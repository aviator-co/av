package timeutils

import (
	"time"
)

// FormalLocal takes a timestamp in string format (inputLayout) and converts it into a readable
// format in the local timezome (outputLayout).
func FormatLocal(timeString string) string {
	inputLayout := "2006-01-02T15:04:05.999999Z07:00"
	outputLayout := "2 January 2006 3:04:05 PM PST"

	timestamp, err := time.Parse(inputLayout, timeString)
	if err != nil {
		return timeString
	}

	timestampDefaultTZ := timestamp.Local()
	return timestampDefaultTZ.Format(outputLayout)
}
