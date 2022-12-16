package editor

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEditor(t *testing.T) {
	res, err := Launch(nil, Config{
		Text:          "Hello world!\n\nBonjour le monde!\n; This is a comment\n",
		CommentPrefix: ";",
		Command:       "true",
	})
	require.NoError(t, err, "failed to launch editor")
	require.Equal(t, "Hello world!\n\nBonjour le monde!\n", res)
}
