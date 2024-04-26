package main

import (
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/spf13/cobra"
)

var stackTreeCmd = &cobra.Command{
	Use:     "tree",
	Aliases: []string{"t"},
	Short:   "show the tree of stacked branches",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		db, err := getDB(repo)
		if err != nil {
			return err
		}
		tx := db.ReadTx()

		var currentBranch string
		if dh, err := repo.DetachedHead(); err != nil {
			return err
		} else if !dh {
			currentBranch, err = repo.CurrentBranchName()
			if err != nil {
				return err
			}
		}

		rootNodes := stackutils.BuildStackTree(repo, tx, currentBranch)
		for _, node := range rootNodes {
			stackutils.PrintNode(0, currentBranch, true, node)
		}
		return nil
	},
}
