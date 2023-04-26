package meta

type DB interface {
	ReadTx() ReadTx
	WriteTx() WriteTx
}

// ReadTx is a transaction that can be used to read from the database.
// It presents a consistent view of the underlying database.
type ReadTx interface {
	// Repository returns the repository information.
	Repository() (Repository, bool)
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
	// The transaction cannot be used after it has been aborted.
	Abort()
	// Commit commits the transaction (all changes will be committed).
	// If an error is returned, the data could not be commited.
	// The transaction cannot be used after it has been committed (even if an
	// error is returned).
	Commit() error
	// SetBranch sets the given branch in the database.
	SetBranch(branch Branch)
	// SetRepository sets the repository information in the database.
	SetRepository(repository Repository)
}
