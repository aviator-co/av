package actions_test

import (
	"fmt"
	"testing"

	"github.com/aviator-co/av/internal/actions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadPRMetadata(t *testing.T) {
	prMeta := actions.PRMetadata{
		Parent:     "foo",
		ParentHead: "bar",
		ParentPull: 123,
		Trunk:      "baz",
	}
	prBody := actions.AddPRMetadata("Hello! This is a cool PR that does some neat things.", prMeta)
	fmt.Println(prBody)
	prMeta2, err := actions.ReadPRMetadata(prBody)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, prMeta.Parent, prMeta2.Parent)
	assert.Equal(t, prMeta.ParentHead, prMeta2.ParentHead)
	assert.Equal(t, prMeta.ParentPull, prMeta2.ParentPull)
	assert.Equal(t, prMeta.Trunk, prMeta2.Trunk)

	prBody = actions.AddPRMetadata(prBody, actions.PRMetadata{
		Parent:     "foo2",
		ParentHead: "bar2",
		ParentPull: 1234,
		Trunk:      "baz2",
	})
	assert.Contains(t, prBody, "Hello! This is a cool PR that does some neat things.\n\n")
	prMeta2, err = actions.ReadPRMetadata(prBody)
	require.NoError(t, err)
	assert.Equal(t, "foo2", prMeta2.Parent)
	assert.Equal(t, "bar2", prMeta2.ParentHead)
}

func TestPRMetadataPreservesBody(t *testing.T) {
	sampleMeta := actions.PRMetadata{
		Parent:     "foo",
		ParentHead: "bar",
		ParentPull: 123,
		Trunk:      "baz",
	}
	body1 := actions.AddPRMetadata(
		"Hello! This is a cool PR that does some neat things.",
		sampleMeta,
	)
	// Add some text to the end of the body (as if someone had edited manually)
	body1 += "\n\nIt's very neat, actually."

	body2 := actions.AddPRMetadata(body1, sampleMeta)
	assert.Contains(t, body2, "Hello! This is a cool PR that does some neat things.")
	assert.Contains(t, body2, "It's very neat, actually.")
	assert.Contains(t, body2, "\n"+actions.PRMetadataCommentStart)
}
