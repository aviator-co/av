package stringutils_test

import (
	"github.com/aviator-co/av/internal/utils/stringutils"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSplitLines(t *testing.T) {
	input := `line1
line2

line4
`
	expected := []string{"line1", "line2", "", "line4"}

	require.Equal(t, expected, stringutils.SplitLines(input))

	input = ""
	expected = []string(nil)

	require.Equal(t, expected, stringutils.SplitLines(input))
}
