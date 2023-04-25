package jsonfiledb

import "github.com/aviator-co/av/internal/meta"

type writeTx struct {
	state
	aborted bool
}

func (tx *writeTx) SetBranch(branch meta.Branch) {
	tx.Branches[branch.Name] = branch
}

func (tx *writeTx) Abort() {
	tx.aborted = true
}

var _ meta.WriteTx = &writeTx{}
