package reorder

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStackBranchCmd_String(t *testing.T) {
	for _, tt := range []struct {
		Cmd    StackBranchCmd
		Output string
	}{
		{StackBranchCmd{Name: "feature-one"}, "stack-branch feature-one"},
		{StackBranchCmd{Name: "feature-one", Parent: "master"}, "stack-branch feature-one --parent master"},
		{StackBranchCmd{Name: "feature-one", Trunk: "master"}, "stack-branch feature-one --trunk master"},
	} {
		t.Run(tt.Output, func(t *testing.T) {
			assert.Equal(t, tt.Output, tt.Cmd.String())
		})
	}
}
