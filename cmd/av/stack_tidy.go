package main

import (
	"fmt"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/textutils"
	"os"
	"strings"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/spf13/cobra"
)

var stackTidyCmd = &cobra.Command{
	Use:   "tidy",
	Short: "tidy up the branch metadata",
	Long: strings.TrimSpace(`
Tidy up the branch metadata by removing the deleted / merged branches.

This command detects which branch is deleted or merged, and re-parent the child branches. This
operates on only av's internal metadata, and it won't delete the actual Git branches.
	`),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		db, err := getDB(repo)
		if err != nil {
			return err
		}
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
		rebuildChildren(branches)

		if err := tx.Commit(); err != nil {
			return err
		}

		if nDeleted > 0 {
			_, _ = fmt.Fprint(os.Stderr,
				"Tidied ", colors.UserInput(nDeleted), " ",
				textutils.Pluralize(nDeleted, "branch", "branches"),
				".\n",
			)
		} else {
			_, _ = fmt.Fprintln(os.Stderr, "No branches to tidy.")
		}
		return nil
	},
}

// findNonDeletedParents finds the non-deleted/merged branch for each deleted/merged branches.
func findNonDeletedParents(repo *git.Repo, branches map[string]*meta.Branch) map[string]meta.BranchState {
	deleted := make(map[string]bool)
	for name, br := range branches {
		if br.MergeCommit != "" {
			deleted[name] = true
		} else if _, err := repo.Git("show-ref", "refs/heads/"+name); err != nil {
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

// rebuildChildren removes Children for all branches and recreates them from Parent.
func rebuildChildren(branches map[string]*meta.Branch) {
	for _, br := range branches {
		br.Children = nil
	}
	for name, br := range branches {
		if parent, ok := branches[br.Parent.Name]; ok {
			parent.Children = append(parent.Children, name)
		}
	}
}
