package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGitVersion(t *testing.T) {
	tests := []struct {
		name      string
		out       string
		wantMajor int
		wantMinor int
		wantErr   bool
	}{
		{"stable", "git version 2.54.0", 2, 54, false},
		{"patch", "git version 2.44.1", 2, 44, false},
		{"apple suffix", "git version 2.45.2 (Apple Git-152)", 2, 45, false},
		{"trailing newline", "git version 2.30.0\n", 2, 30, false},
		{"bad prefix", "hg version 2.30.0", 0, 0, true},
		{"missing minor", "git version 2", 0, 0, true},
		{"non-numeric", "git version foo.bar", 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			major, minor, err := parseGitVersion(tt.out)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantMajor, major)
			assert.Equal(t, tt.wantMinor, minor)
		})
	}
}
