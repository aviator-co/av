package logutils

import (
	"fmt"
)

// FormatPrinter is a simple wrapper that implements the Stringer interface by
// printing an arbitrary object with a given format specifier/verb.
type FormatPrinter struct {
	verb string
	item any
}

func (v FormatPrinter) String() string {
	return fmt.Sprintf(v.verb, v.item)
}

func Format(verb string, item any) FormatPrinter {
	return FormatPrinter{verb, item}
}
