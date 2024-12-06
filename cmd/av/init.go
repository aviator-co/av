package main

import (
	"context"
	"fmt"

	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/cleanup"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the repository for Aviator CLI",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) (reterr error) {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		db, _, err := getOrCreateDB(repo)
		if err != nil {
			return err
		}
		tx := db.WriteTx()
		cu := cleanup.New(func() {
			logrus.WithError(reterr).Debug("aborting db transaction")
			tx.Abort()
		})
		defer cu.Cleanup()

		client, err := getGitHubClient()
		if err != nil {
			return err
		}

		origin, err := repo.Origin()
		if err != nil {
			return err
		}

		ghRepo, err := client.GetRepositoryBySlug(context.Background(), origin.RepoSlug)
		if err != nil {
			return err
		}

		tx.SetRepository(meta.Repository{
			ID:    ghRepo.ID,
			Owner: ghRepo.Owner.Login,
			Name:  ghRepo.Name,
		})

		cu.Cancel()
		if err := tx.Commit(); err != nil {
			return err
		}
		fmt.Println("Successfully initialized repository for use with av!")
		return nil
	},
}
