package main

import (
	"fmt"
	"os"
	"slices"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/spf13/cobra"
)

var syncExcludeFlags struct {
	List bool
}

var syncExcludeCmd = &cobra.Command{
	Use:   "sync-exclude [<branch>]",
	Short: "Toggle whether a branch is excluded from sync --all",
	Long: `Toggle whether a branch is excluded from "av sync --all" operations.

When a branch is excluded, it will be skipped during "av sync --all" but can
still be synced by explicitly naming it or by running sync from within the stack.

Running this command on a branch toggles its exclusion state:
- If the branch is currently excluded, it will be included
- If the branch is currently included, it will be excluded

Use --list to see all currently excluded branches.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: branchNameArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		repo, err := getRepo(ctx)
		if err != nil {
			return err
		}
		db, err := getDB(ctx, repo)
		if err != nil {
			return err
		}

		if syncExcludeFlags.List {
			return handleListExcluded(db)
		}

		if len(args) == 0 {
			return errors.New("branch name required (or use --list to see excluded branches)")
		}

		return toggleBranchExclusion(db, args[0])
	},
}

func init() {
	syncExcludeCmd.Flags().BoolVar(
		&syncExcludeFlags.List, "list", false,
		"list all branches excluded from sync --all",
	)
}

func toggleBranchExclusion(db meta.DB, branchName string) error {
	tx := db.WriteTx()
	defer tx.Abort()

	branch, exists := tx.Branch(branchName)
	if !exists {
		return errors.Errorf("branch %q is not adopted by av", branchName)
	}

	// Count descendants before toggling
	descendants := meta.SubsequentBranches(tx, branchName)
	descendantCount := len(descendants)

	branch.ExcludeFromSyncAll = !branch.ExcludeFromSyncAll
	tx.SetBranch(branch)
	if err := tx.Commit(); err != nil {
		return errors.WrapIf(err, "failed to update branch metadata")
	}

	if branch.ExcludeFromSyncAll {
		_, _ = os.Stderr.WriteString(colors.Success(fmt.Sprintf("Branch %q is now excluded from sync --all\n", branchName)))
		if descendantCount > 0 {
			_, _ = os.Stderr.WriteString(colors.Faint(fmt.Sprintf("Note: %d descendant branch(es) will also be excluded\n", descendantCount)))
		}
	} else {
		_, _ = os.Stderr.WriteString(colors.Success(fmt.Sprintf("Branch %q is now included in sync --all\n", branchName)))
		if descendantCount > 0 {
			_, _ = os.Stderr.WriteString(colors.Faint(fmt.Sprintf("Note: %d descendant branch(es) will also be included\n", descendantCount)))
		}
	}
	return nil
}

func handleListExcluded(db meta.DB) error {
	tx := db.ReadTx()

	branches := tx.AllBranches()
	var excluded []string
	for _, branch := range branches {
		if branch.ExcludeFromSyncAll {
			excluded = append(excluded, branch.Name)
		}
	}

	if len(excluded) == 0 {
		_, _ = os.Stderr.WriteString("No branches are excluded from sync --all\n")
		return nil
	}

	slices.Sort(excluded)

	_, _ = os.Stderr.WriteString("Branches excluded from sync --all:\n")
	for _, branchName := range excluded {
		_, _ = os.Stderr.WriteString(colors.Faint(fmt.Sprintf("  - %s\n", branchName)))
	}

	return nil
}
