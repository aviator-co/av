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

	require.Equal(t,
		StackBranchCmd{Name: "one", Trunk: "main@" + fullTrunk},
		resolveHashCmd(StackBranchCmd{Name: "one", Trunk: "main@aaaaaaa"}, shortToFull),
	)
	require.Equal(t,
		PickCmd{Commit: fullPick},
		resolveHashCmd(PickCmd{Commit: "bbbbbbb"}, shortToFull),
	)
}
