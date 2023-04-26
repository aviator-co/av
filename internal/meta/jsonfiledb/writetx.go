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
	tx.state.BranchState[branch.Name] = branch
}

func (tx *writeTx) Abort() {
	tx.db.stateMu.Unlock()
	tx.db = nil
}

func (tx *writeTx) Commit() error {
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
