package jsonfiledb

import (
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/maputils"
	"golang.org/x/exp/slices"
)

type readTx struct {
	state state
}

var _ meta.ReadTx = &readTx{}

func (tx *readTx) Repository() (meta.Repository, bool) {
	return tx.state.RepositoryState, tx.state.RepositoryState.ID != ""
}

func (tx *readTx) Branch(name string) (branch meta.Branch, ok bool) {
	branch, ok = tx.state.BranchState[name]
	if !ok {
		branch.Name = name
	}
	return
}

func (tx *readTx) AllBranches() map[string]meta.Branch {
	return maputils.Copy(tx.state.BranchState)
}

func (tx *readTx) Children(name string) []meta.Branch {
	var children []meta.Branch
	for _, branch := range tx.state.BranchState {
		if branch.Parent.Name == name {
			children = append(children, branch)
		}
	}

	// Sort for deterministic output.
	slices.SortFunc(children, func(a, b meta.Branch) bool {
		return a.Name < b.Name
	})
	return children
}
