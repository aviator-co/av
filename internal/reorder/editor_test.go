package reorder

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEditorHashMapping(t *testing.T) {
	fullTrunk := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	fullPick := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	shortToFull := make(map[string]string)
	for _, cmd := range []Cmd{
		StackBranchCmd{Name: "one", Trunk: "main@" + fullTrunk},
		PickCmd{Commit: fullPick},
	} {
		cmd.EditorString(shortToFull)
	}

	require.Equal(t, fullTrunk, shortToFull["aaaaaaa"])
	require.Equal(t, fullPick, shortToFull["bbbbbbb"])
}
