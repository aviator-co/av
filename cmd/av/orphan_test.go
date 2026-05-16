package main

import (
	"testing"

	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestOrphanFromTrunkDeniedPromptDoesNotOrphanBranches(t *testing.T) {
	repo := gittest.NewTempRepo(t)
	db := repo.OpenDB(t)
	tx := db.WriteTx()
	tx.SetBranch(meta.Branch{
		Name: "stack-1",
		Parent: meta.BranchState{
			Name:  "main",
			Trunk: true,
		},
	})
	tx.SetBranch(meta.Branch{
		Name: "stack-2",
		Parent: meta.BranchState{
			Name: "stack-1",
		},
	})
	require.NoError(t, tx.Commit())

	branchesToOrphan, err := collectBranchesToOrphan(repo.AsAvGitRepo(), db.ReadTx())
	require.NoError(t, err)
	require.Equal(t, []string{"main", "stack-1", "stack-2"}, branchesToOrphan)

	vm := &orphanConfirmViewModel{
		db:               db,
		branchesToOrphan: branchesToOrphan,
	}
	if cmd := vm.Init(); cmd != nil {
		_ = cmd()
	}

	_, cmd := vm.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		_ = cmd()
	}
	_, cmd = vm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		_ = cmd()
	}

	_, ok := db.ReadTx().Branch("stack-1")
	require.True(t, ok, "stack-1 should remain adopted after denying the prompt")
	_, ok = db.ReadTx().Branch("stack-2")
	require.True(t, ok, "stack-2 should remain adopted after denying the prompt")
}
