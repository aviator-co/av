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
	rebuildChildren bool
	trunk           bool
	parent          string
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
		branchMetaRebuildChildrenCmd,
		branchMetaSetCmd,
	)
}

var branchMetaDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete a branch metadata",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}
		for _, branch := range args {
			if err := meta.DeleteBranch(repo, branch); err != nil {
				return err
			}
		}
		if branchMetaFlags.rebuildChildren {
			if err := meta.RebuildChildren(repo); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	branchMetaDeleteCmd.Flags().BoolVar(
		&branchMetaFlags.rebuildChildren, "rebuild-children", true,
		"rebuild children fields based on parent after modifying the branch metadata",
	)
}

var branchMetaListCmd = &cobra.Command{
	Use:   "list",
	Short: "list all branch metadata",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}
		branches, err := meta.ReadAllBranches(repo)
		if err != nil {
			return err
		}
		bs, err := json.MarshalIndent(branches, "", "    ")
		if err != nil {
			return err
		}
		fmt.Println(string(bs))
		return nil
	},
}

var branchMetaRebuildChildrenCmd = &cobra.Command{
	Use:   "rebuild-children",
	Short: "rebuild the children based on the parent",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}
		if err := meta.RebuildChildren(repo); err != nil {
			return err
		}
		return nil
	},
}

var branchMetaSetCmd = &cobra.Command{
	Use:   "set",
	Short: "modify the branch metadata",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			_ = cmd.Usage()
			return errors.New("exactly one branch name and --parent is required")
		}
		repo, err := getRepo()
		if err != nil {
			return err
		}
		if _, err := repo.RevParse(&git.RevParse{Rev: args[0]}); err != nil {
			return errors.WrapIf(err, "cannot check if a branch exists")
		}
		br, _ := meta.ReadBranch(repo, args[0])
		if branchMetaFlags.parent != "" {
			br.Parent, err = meta.ReadBranchState(repo, branchMetaFlags.parent, branchMetaFlags.trunk)
			if err != nil {
				return err
			}
		}
		if err := meta.WriteBranch(repo, br); err != nil {
			return errors.WrapIff(err, "failed to write av internal metadata for branch %q", branchMetaFlags.parent)
		}

		if branchMetaFlags.rebuildChildren {
			if err := meta.RebuildChildren(repo); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	branchMetaSetCmd.Flags().BoolVar(
		&branchMetaFlags.rebuildChildren, "rebuild-children", true,
		"rebuild children fields based on parent after modifying the branch metadata",
	)
	branchMetaSetCmd.Flags().BoolVar(
		&branchMetaFlags.trunk, "trunk", false,
		"mark the parent branch as trunk",
	)
	branchMetaSetCmd.Flags().StringVar(
		&branchMetaFlags.parent, "parent", "",
		"parent branch name",
	)
}
