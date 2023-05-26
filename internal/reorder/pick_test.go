package reorder

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPickCmd_String(t *testing.T) {
	assert.Equal(t, "pick mycommit", PickCmd{Commit: "mycommit"}.String())
}
