package refmeta

import (
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/cleanup"
	"github.com/sirupsen/logrus"
)

// Import imports all ref metadata from the git repo into the database.
func Import(repo *git.Repo, db meta.DB) error {
	tx := db.WriteTx()
	cu := cleanup.New(func() { tx.Abort() })
	defer cu.Cleanup()

	repoMeta, err := ReadRepository(repo)
	if err != nil {
		return err
	}
	tx.SetRepository(repoMeta)

	allBranchMetas, err := ReadAllBranches(repo)
	if err != nil {
		return err
	}
	for _, branchMeta := range allBranchMetas {
		tx.SetBranch(branchMeta)
	}
	logrus.
		WithField("branches", len(allBranchMetas)).
		Debug("Imported branches into av database from ref metadata")

	cu.Cancel()
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
