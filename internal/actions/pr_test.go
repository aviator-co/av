package actions_test

import (
	"github.com/aviator-co/av/internal/actions"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestReadPRMetadata(t *testing.T) {
	prMeta := actions.PRMetadata{
		Parent:     "foo",
		ParentHead: "bar",
		Trunk:      "baz",
	}
	prBody := "Hello! This is a cool PR that does some neat things.\n\n" + actions.WritePRMetadata(prMeta)
	prMeta2, err := actions.ReadPRMetadata(prBody)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, prMeta.Parent, prMeta2.Parent)
	assert.Equal(t, prMeta.ParentHead, prMeta2.ParentHead)
	assert.Equal(t, prMeta.Trunk, prMeta2.Trunk)
}
