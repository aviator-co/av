package actions

import (
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
)

// TidyDB removes deleted branches from the metadata and returns number of branches removed from the
// DB.
func TidyDB(repo *git.Repo, db meta.DB) (map[string]bool, map[string]bool, error) {
	tx := db.WriteTx()
	defer tx.Abort()
	branches := tx.AllBranches()

	deleted := make(map[string]bool)
	for name := range branches {
		if _, err := repo.Git("show-ref", "refs/heads/"+name); err != nil {
			// Ref doesn't exist. Should be removed.
			deleted[name] = true
		}
	}
	orphaned := make(map[string]bool)
	for name := range branches {
		if deleted[name] {
			continue
		}
		if isParentDeleted(branches, deleted, name) {
			orphaned[name] = true
		}
	}

	for name := range deleted {
		tx.DeleteBranch(name)
	}
	for name := range orphaned {
		tx.DeleteBranch(name)
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	return deleted, orphaned, nil
}

func isParentDeleted(branches map[string]meta.Branch, deleted map[string]bool, branch string) bool {
	state := branches[branch].Parent
	for !state.Trunk {
		if deleted[state.Name] {
			return true
		}
		state = branches[state.Name].Parent
	}
	return false
}
