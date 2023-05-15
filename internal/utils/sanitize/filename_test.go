package sanitize

import (
	"fmt"
	"testing"
)

func TestFileName(t *testing.T) {
	for _, tt := range []struct{ Input, Output string }{
		{"", ""},
		{"  hello    world ", "hello-world"},
		{"hello world", "hello-world"},
		{"hello-world", "hello-world"},
		{"hello_world", "hello-world"},
		{"hello-world-123", "hello-world-123"},
		{"hello-/-world-123!!!", "hello-world-123"},
	} {
		name := fmt.Sprintf("%q=>%q", tt.Input, tt.Output)
		t.Run(name, func(t *testing.T) {
			if got := FileName(tt.Input); got != tt.Output {
				t.Errorf("FileName() = %q, want %q", got, tt.Output)
			}
		})
	}
}
