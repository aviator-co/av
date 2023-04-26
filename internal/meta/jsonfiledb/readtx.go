package jsonfiledb

import (
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/maputils"
)

type readTx struct {
	state state
}

var _ meta.ReadTx = &readTx{}

func (tx *readTx) Branch(name string) (branch meta.Branch, ok bool) {
	branch, ok = tx.state.Branches[name]
	return
}

func (tx *readTx) AllBranches() map[string]meta.Branch {
	return maputils.Copy(tx.state.Branches)
}
