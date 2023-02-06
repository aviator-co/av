package editor

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEditor(t *testing.T) {
	type test struct {
		name    string
		command string
		in      string
		out     string
		error   bool
	}
	for _, tt := range []test{
		{
			name:    "with comments",
			command: "true",
			in:      "Hello world!\n\nBonjour le monde!\n%% This is a commend\n",
			out:     "Hello world!\n\nBonjour le monde!\n",
		},
		{
			name:    "command with flags",
			command: "sed -i -e 's/Hello/Hi/'",
			in:      "Hello world!\n\nBonjour le monde!\n",
			out:     "Hi world!\n\nBonjour le monde!\n",
		},
		{
			name:    "error",
			command: "true",
			in:      "Hello world!\n\nBonjour le monde!\n",
			error:   true,
		},
	} {
		res, err := Launch(nil, Config{
			Text:          tt.in,
			CommentPrefix: "%%",
			Command:       tt.command,
		})
		if tt.error {
			require.Error(t, err, "expected error while executing `%s`", tt.command)
			continue
		}
		require.NoError(t, err, "failed to launch editor `%s`", tt.command)
		require.Equal(t, tt.out, res)
	}
}
