package jsonfiledb

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/aviator-co/av/internal/meta"
)

type DB struct {
	filepath string

	stateMu sync.Mutex
	state   *state
}

// OpenPath opens a JSON file database at the given path.
// If the file does not exist, it is created (as well as all ancestor directories).
func OpenPath(fp string) (*DB, bool, error) {
	_ = os.MkdirAll(filepath.Dir(fp), 0o755)
	state, err := readState(fp)
	if err != nil {
		return nil, false, err
	}
	db := &DB{filepath: fp, stateMu: sync.Mutex{}, state: state}
	return db, state.RepositoryState.ID != "", nil
}

func (d *DB) ReadTx() meta.ReadTx {
	// Acquire the lock in order to safely access and copy state, but we don't
	// need to hold the lock for the entire duration of the read transaction.
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	return &readTx{d.state.copy()}
}

func (d *DB) WriteTx() meta.WriteTx {
	// For a write transaction, we acquire the lock until the transaction is
	// aborted/committed in order to prevent other transactions from modifying
	// the state.
	d.stateMu.Lock()
	return &writeTx{d, readTx{d.state.copy()}}
}

var _ meta.DB = &DB{}
