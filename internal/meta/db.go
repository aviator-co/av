package meta

type DB interface {
	ReadTx() ReadTx
	WriteTx() WriteTx
}

// ReadTx is a transaction that can be used to read from the database.
// It presents a consistent view of the underlying database.
type ReadTx interface {
	// Repository returns the repository information.
	Repository() Repository
	// Branch returns the branch with the given name. If no such branch exists,
	// the second return value is false.
	Branch(name string) (Branch, bool)
	// AllBranches returns a map of all branches in the database.
	AllBranches() map[string]Branch
}

// WriteTx is a transaction that can be used to modify the database.
// The transaction MUST be finalized by calling either Abort or Commit.
type WriteTx interface {
	ReadTx
	// Abort finalizes the transaction without committing any changes.
	// Abort can be called even after the transaction has been finalized (which
	// is effectively a no-op).
	Abort()
	// Commit finalizes the transaction and commits all changes.
	// If an error is returned, the data could not be committed.
	// Commit will panic if called after the transaction has been finalized.
	Commit() error
	// SetBranch sets the given branch in the database.
	SetBranch(branch Branch)
	// DeleteBranch deletes the given branch in the database.
	DeleteBranch(name string)
	// SetRepository sets the repository information in the database.
	SetRepository(repository Repository)
}
