package main

import (
	"fmt"
	"os"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/meta"
	"github.com/spf13/cobra"
)

var orphanCmd = &cobra.Command{
	Use:   "orphan",
	Short: "Orphan branches that are managed by av",
	Long: strings.TrimSpace(`
Orphan the currently checked-out branch and any child branches that are managed by av.

When run from the trunk branch, this orphans every av-managed branch because all managed
branches descend from trunk. A separate warning for this behavior is tracked in #733.

To manage orphaned branches with av again, re-adopt them one at a time with "av adopt".`),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		repo, err := getRepo(ctx)
		if err != nil {
			return err
		}

		db, err := getDB(ctx, repo)
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

		fmt.Fprintf(
			os.Stderr,
			"These branched are orphaned: %s\n",
			strings.Join(branchesToOrphan, ", "),
		)

		return nil
	},
}
