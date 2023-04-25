package meta

type DB interface {
	ReadTx
	// WithTx runs the given function in a transaction. The transaction is
	// committed when the function returns (unless it has been aborted by
	// calling WriteTx.Abort).
	WithTx(func(tx WriteTx)) error
}

type ReadTx interface {
	// Branch returns the branch with the given name. If no such branch exists,
	// the second return value is false.
	Branch(name string) (Branch, bool)
	// AllBranches returns a map of all branches in the database.
	AllBranches() map[string]Branch
}

// WriteTx is a transaction that can be used to modify the database.
type WriteTx interface {
	ReadTx
	// Abort aborts the transaction (no changes will be committed).
	Abort()
	// SetBranch sets the given branch in the database.
	SetBranch(branch Branch)
}
