package actions

import (
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
)

// TidyDB removes deleted branches from the metadata and returns number of branches removed from the
// DB.
func TidyDB(repo *git.Repo, db meta.DB) (int, error) {
	tx := db.WriteTx()
	defer tx.Abort()
	origBranches := tx.AllBranches()
	branches := make(map[string]*meta.Branch)
	for name, br := range origBranches {
		// origBranches has values, not references. Convert to references so that we
		// can modify them through references.
		b := br
		branches[name] = &b
	}

	newParents := findNonDeletedParents(repo, branches)
	for name, br := range branches {
		if _, deleted := newParents[name]; deleted {
			// This branch is merged/deleted. Do not have to change the parent.
			continue
		}
		if newParent, ok := newParents[br.Parent.Name]; ok {
			br.Parent = newParent
		}
	}

	nDeleted := 0
	for name, br := range branches {
		if _, deleted := newParents[name]; deleted {
			tx.DeleteBranch(name)
			nDeleted += 1
			continue
		}
		tx.SetBranch(*br)
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return nDeleted, nil
}

// findNonDeletedParents finds the non-deleted/merged branch for each deleted/merged branches.
func findNonDeletedParents(
	repo *git.Repo,
	branches map[string]*meta.Branch,
) map[string]meta.BranchState {
	deleted := make(map[string]bool)
	for name := range branches {
		if _, err := repo.Git("show-ref", "refs/heads/"+name); err != nil {
			// Ref doesn't exist. Should be removed.
			deleted[name] = true
		}
	}

	liveParents := make(map[string]meta.BranchState)
	for name := range deleted {
		state := branches[name].Parent
		for !state.Trunk && deleted[state.Name] {
			state = branches[state.Name].Parent
		}
		liveParents[name] = state
	}
	return liveParents
}
