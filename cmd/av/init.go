package main

import (
	"context"
	"fmt"

	"github.com/aviator-co/av/internal/utils/cleanup"
	"github.com/sirupsen/logrus"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/meta"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use: "init",
	RunE: func(cmd *cobra.Command, args []string) (reterr error) {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		db, err := getDB(repo)
		if err != nil {
			return err
		}
		tx := db.WriteTx()
		cu := cleanup.New(func() {
			logrus.WithError(reterr).Debug("aborting db transaction")
			tx.Abort()
		})
		defer cu.Cleanup()

		if config.Av.GitHub.Token == "" {
			return errors.New("github token must be set")
		}
		client, err := getClient(config.Av.GitHub.Token)
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
		_, _ = fmt.Println("Successfully initialized repository for use with av!")
		return nil
	},
}
