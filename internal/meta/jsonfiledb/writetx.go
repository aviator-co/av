package jsonfiledb

import "github.com/aviator-co/av/internal/meta"

type writeTx struct {
	db *DB
	readTx
}

func (tx *writeTx) SetRepository(repository meta.Repository) {
	tx.state.RepositoryState = repository
}

func (tx *writeTx) SetBranch(branch meta.Branch) {
	if branch.Name == "" {
		panic("cannot set branch with empty name")
	}
	tx.state.BranchState[branch.Name] = branch
}

func (tx *writeTx) DeleteBranch(name string) {
	delete(tx.state.BranchState, name)
}

func (tx *writeTx) Abort() {
	// Abort after finalize is a no-op.
	if tx.db == nil {
		return
	}
	tx.db.stateMu.Unlock()
	tx.db = nil
}

func (tx *writeTx) Commit() error {
	if tx.db == nil {
		panic("cannot commit transaction: already finalized")
	}
	// Always unlock the database even if there is an error.
	defer tx.db.stateMu.Unlock()
	err := tx.state.write(tx.db.filepath)
	if err != nil {
		return err
	}
	*tx.db.state = tx.state
	tx.db = nil
	return nil
}

var _ meta.WriteTx = &writeTx{}
