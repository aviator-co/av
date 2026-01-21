package reorder

import (
	"bytes"
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestStackBranchCmd_ExecuteRejectsCycle(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	db := repo.OpenDB(t)
	out := &bytes.Buffer{}
	ctx := &Context{repo.AsAvGitRepo(), db, &State{Branch: "main"}, out}

	repo.Git(t, "switch", "-c", "child")
	repo.CommitFile(t, "child.txt", "child\n")
	repo.Git(t, "switch", "main")

	tx := db.WriteTx()
	tx.SetBranch(meta.Branch{
		Name: "parent",
		Parent: meta.BranchState{
			Name:  "main",
			Trunk: true,
		},
	})
	tx.SetBranch(meta.Branch{
		Name: "child",
		Parent: meta.BranchState{
			Name:  "parent",
			Trunk: false,
		},
	})
	require.NoError(t, tx.Commit())

	err := StackBranchCmd{Name: "parent", Parent: "child"}.Execute(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cyclical branch dependencies")
}
