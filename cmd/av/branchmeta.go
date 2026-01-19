package main

import (
	"encoding/json"
	"fmt"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/spf13/cobra"
)

var branchMetaFlags struct {
	trunk  bool
	parent string
}

var branchMetaCmd = &cobra.Command{
	Use:    "branch-meta",
	Short:  "low-level command to manage branch metadata",
	Hidden: true,
}

func init() {
	branchMetaCmd.AddCommand(
		branchMetaDeleteCmd,
		branchMetaListCmd,
		branchMetaSetCmd,
	)
}

var branchMetaDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete a branch metadata",
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
		tx := db.WriteTx()
		defer tx.Abort()
		for _, branch := range args {
			tx.DeleteBranch(branch)
		}
		return tx.Commit()
	},
}

var branchMetaListCmd = &cobra.Command{
	Use:   "list",
	Short: "list all branch metadata",
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
		tx := db.ReadTx()
		branches := tx.AllBranches()
		bs, err := json.MarshalIndent(branches, "", "    ")
		if err != nil {
			return err
		}
		fmt.Println(string(bs))
		return nil
	},
}

var branchMetaSetCmd = &cobra.Command{
	Use:   "set branch-name",
	Short: "modify the branch metadata",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if len(args) != 1 {
			_ = cmd.Usage()
			return errors.New("exactly one branch name and --parent is required")
		}
		repo, err := getRepo(ctx)
		if err != nil {
			return err
		}
		db, err := getDB(ctx, repo)
		if err != nil {
			return err
		}
		if _, err := repo.RevParse(ctx, &git.RevParse{Rev: args[0]}); err != nil {
			return errors.WrapIf(err, "cannot check if a branch exists")
		}
		tx := db.WriteTx()
		defer tx.Abort()
		br, _ := tx.Branch(args[0])
		if branchMetaFlags.parent != "" {
			var parentHead string
			if branchMetaFlags.trunk {
				var err error
				parentHead, err = repo.RevParse(ctx, &git.RevParse{Rev: branchMetaFlags.parent})
				if err != nil {
					return err
				}
			}
			br.Parent = meta.BranchState{
				Name:                     branchMetaFlags.parent,
				Trunk:                    branchMetaFlags.trunk,
				BranchingPointCommitHash: parentHead,
			}
			if err := meta.ValidateNoCycle(tx, args[0], br.Parent); err != nil {
				return fmt.Errorf(
					"could not set parent for branch %q because it would introduce cyclical branch dependencies",
					args[0],
				)
			}
		}
		tx.SetBranch(br)
		return tx.Commit()
	},
}

func init() {
	branchMetaSetCmd.Flags().BoolVar(
		&branchMetaFlags.trunk, "trunk", false,
		"mark the parent branch as trunk",
	)
	branchMetaSetCmd.Flags().StringVar(
		&branchMetaFlags.parent, "parent", "",
		"parent branch name",
	)
}
