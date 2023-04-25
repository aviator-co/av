package jsonfiledb

import (
	"github.com/aviator-co/av/internal/meta"
)

type DB struct {
	*state
	filepath string
}

// Open opens a JSON file database at the given path.
// If the file does not exist, it is created.
func Open(filepath string) (*DB, error) {
	state, err := readState(filepath)
	if err != nil {
		return nil, err
	}
	db := &DB{state, filepath}
	return db, nil
}

func (d *DB) WithTx(fn func(tx meta.WriteTx)) error {
	// Make a copy of the state.
	// Since all of the properties are immutable trees, this is effectively a
	// deep copy.
	tx := &writeTx{state: d.state.copy()}
	fn(tx)
	if tx.aborted {
		return nil
	}
	if err := tx.state.write(d.filepath); err != nil {
		return err
	}
	d.state = &tx.state
	return nil
}

var (
	_ meta.DB = &DB{}
)
