package jsonfiledb

import (
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"os"
	"path"
	"sync"
)

type DB struct {
	filepath string

	stateMu sync.Mutex
	state   *state
}

func RepoPath(repo *git.Repo) string {
	return path.Join(repo.AvDir(), "av.db")
}

func OpenRepo(repo *git.Repo) (*DB, error) {
	return OpenPath(RepoPath(repo))
}

// OpenPath opens a JSON file database at the given path.
// If the file does not exist, it is created (as well as all ancestor directories).
func OpenPath(filepath string) (*DB, error) {
	_ = os.MkdirAll(path.Dir(filepath), 0755)
	state, err := readState(filepath)
	if err != nil {
		return nil, err
	}
	db := &DB{filepath, sync.Mutex{}, state}
	return db, nil
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

var (
	_ meta.DB = &DB{}
)
