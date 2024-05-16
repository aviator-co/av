package jsonfiledb

import (
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/maputils"
)

type readTx struct {
	state state
}

var _ meta.ReadTx = &readTx{}

func (tx *readTx) Repository() meta.Repository {
	return tx.state.RepositoryState
}

func (tx *readTx) Branch(name string) (meta.Branch, bool) {
	if name == "" {
		panic("invariant error: cannot read branch state for empty branch name")
	}
	branch, ok := tx.state.BranchState[name]
	if branch.Name == "" {
		branch.Name = name
	}
	return branch, ok
}

func (tx *readTx) AllBranches() map[string]meta.Branch {
	return maputils.Copy(tx.state.BranchState)
}
