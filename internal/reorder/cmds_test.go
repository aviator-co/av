package reorder

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestState(t *testing.T) {
	trunkCommit := "1111111111111111111111111111111111111111"
	firstPick := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	secondPick := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	state := &State{
		Branch: "main",
		Head:   "owouwu",
		Commands: []Cmd{
			StackBranchCmd{Name: "one", Trunk: "main@" + trunkCommit},
			PickCmd{Commit: firstPick},
			StackBranchCmd{Name: "two", Parent: "one"},
			PickCmd{Commit: secondPick},
		},
	}

	serialized, err := json.Marshal(state)
	require.NoError(t, err, "failed to serialize state")

	var deserialized State
	err = json.Unmarshal(serialized, &deserialized)
	require.NoError(t, err, "failed to deserialize state")

	require.Equal(t, *state, deserialized, "deserialized command sequence does not match original")
}

// TestStateRoundTrip_SquashFixupModes verifies that PickCmd with squash and
// fixup modes survive a JSON marshal/unmarshal cycle with the correct Mode
// field restored.
func TestStateRoundTrip_SquashFixupModes(t *testing.T) {
	pickCommit := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	squashCommit := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	fixupCommit := "cccccccccccccccccccccccccccccccccccccccc"
	state := &State{
		Branch: "feature",
		Head:   "deadbeef",
		Commands: []Cmd{
			PickCmd{Commit: pickCommit},
			PickCmd{Commit: squashCommit, Mode: PickModeSquash},
			PickCmd{Commit: fixupCommit, Mode: PickModeFixup},
		},
	}

	serialized, err := json.Marshal(state)
	require.NoError(t, err, "failed to serialize state with squash/fixup modes")

	var deserialized State
	err = json.Unmarshal(serialized, &deserialized)
	require.NoError(t, err, "failed to deserialize state with squash/fixup modes")

	require.Len(t, deserialized.Commands, 3)

	pick, ok := deserialized.Commands[0].(PickCmd)
	require.True(t, ok, "commands[0] should be a PickCmd")
	require.Equal(t, pickCommit, pick.Commit)
	require.Equal(t, PickModePick, pick.Mode, "plain pick mode should round-trip as PickModePick")

	squash, ok := deserialized.Commands[1].(PickCmd)
	require.True(t, ok, "commands[1] should be a PickCmd")
	require.Equal(t, squashCommit, squash.Commit)
	require.Equal(t, PickModeSquash, squash.Mode, "squash mode should survive round-trip")

	fixup, ok := deserialized.Commands[2].(PickCmd)
	require.True(t, ok, "commands[2] should be a PickCmd")
	require.Equal(t, fixupCommit, fixup.Commit)
	require.Equal(t, PickModeFixup, fixup.Mode, "fixup mode should survive round-trip")
}
