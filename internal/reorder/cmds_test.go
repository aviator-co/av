package reorder

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestState(t *testing.T) {
	state := &State{
		Branch: "main",
		Head:   "owouwu",
		Commands: []Cmd{
			StackBranchCmd{Name: "one", Trunk: "main"},
			PickCmd{"abcd"},
			StackBranchCmd{Name: "two", Parent: "one"},
			PickCmd{"efgh"},
		},
	}

	serialized, err := json.Marshal(state)
	require.NoError(t, err, "failed to serialize state")

	var deserialized State
	err = json.Unmarshal(serialized, &deserialized)
	require.NoError(t, err, "failed to deserialize state")

	require.Equal(t, *state, deserialized, "deserialized command sequence does not match original")
}
