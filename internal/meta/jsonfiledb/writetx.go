package jsonfiledb

import (
	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/maputils"
)

type writeTx struct {
	db *DB
	readTx

	// ops records every change in order. Commit replays them on the state
	// currently on disk, so writes from another av process that committed
	// after this transaction started are kept (issue 789).
	ops []func(*state)
}

func (tx *writeTx) apply(op func(*state)) {
	op(&tx.state)
	tx.ops = append(tx.ops, op)
}

func (tx *writeTx) SetRepository(repository meta.Repository) {
	tx.apply(func(s *state) { s.RepositoryState = repository })
}

func (tx *writeTx) SetBranch(branch meta.Branch) {
	if branch.Name == "" {
		panic("cannot set branch with empty name")
	}
	tx.apply(func(s *state) {
		if s.BranchState == nil {
			s.BranchState = map[string]meta.Branch{}
		}
		s.BranchState[branch.Name] = branch
	})
}

func (tx *writeTx) DeleteBranch(name string) {
	tx.apply(func(s *state) { delete(s.BranchState, name) })
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
	// Always finalize the transaction and unlock the database, even if there
	// is an error, so a later Abort does not unlock the mutex a second time.
	db := tx.db
	tx.db = nil
	defer db.stateMu.Unlock()

	// Lock, read and write the real file if av.db is a symlink.
	fp := resolveStatePath(db.filepath)

	// The file lock serializes commits across av processes, for example in
	// two git worktrees that share one av.db.
	unlock, err := lockFile(fp)
	if err != nil {
		return err
	}
	defer unlock()

	fresh, err := readState(fp)
	if err != nil {
		return errors.WrapIff(err, "failed to re-read av state file before commit")
	}
	fresh.BranchState = maputils.Copy(fresh.BranchState)
	if len(tx.ops) > 0 {
		for _, op := range tx.ops {
			op(fresh)
		}
		if err := fresh.write(fp); err != nil {
			return err
		}
	}
	*db.state = *fresh
	return nil
}

var _ meta.WriteTx = &writeTx{}
