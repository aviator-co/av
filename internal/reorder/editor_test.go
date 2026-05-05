package reorder

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEditorHashMapping(t *testing.T) {
	fullTrunk := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	fullPick := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	plan := []Cmd{
		StackBranchCmd{Name: "one", Trunk: "main@" + fullTrunk},
		PickCmd{Commit: fullPick},
	}

	shortToFull := shortHashMap(plan)

	require.Equal(t, fullTrunk, shortToFull["aaaaaaa"])
	require.Equal(t, fullPick, shortToFull["bbbbbbb"])
}
