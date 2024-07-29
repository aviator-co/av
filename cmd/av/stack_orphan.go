package main

import (
	"fmt"
	"os"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/meta"
	"github.com/spf13/cobra"
)

var stackOrphanCmd = &cobra.Command{
	Use:   "orphan",
	Short: "Orphan branches that are managed by av",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
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

		currentBranch, err := repo.CurrentBranchName()
		if err != nil {
			return errors.WrapIf(err, "failed to determine current branch")
		}

		subsequentBranches := meta.SubsequentBranches(tx, currentBranch)

		var branchesToOrphan []string
		branchesToOrphan = append(branchesToOrphan, currentBranch)
		branchesToOrphan = append(branchesToOrphan, subsequentBranches...)

		for _, branch := range branchesToOrphan {
			tx.DeleteBranch(branch)
		}

		if err := tx.Commit(); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "These branched are orphaned: %s\n", strings.Join(branchesToOrphan, ", "))

		return nil
	},
}
