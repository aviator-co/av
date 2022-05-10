package main

import (
	"context"
	"emperror.dev/errors"
	"fmt"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/gh"
	"github.com/aviator-co/av/internal/meta"
	"github.com/spf13/cobra"
)

var initFlags struct {
	Force bool
}
var initCmd = &cobra.Command{
	Use: "init",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		if !initFlags.Force {
			_, ok := meta.ReadRepository(repo)
			if ok {
				return errors.New("repository is already initialized for use with av")
			}
		}

		if config.GitHub.Token == "" {
			return errors.New("github token must be set")
		}
		client, err := gh.NewClient(config.GitHub.Token)
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

		if err := meta.WriteRepository(repo, meta.Repository{
			ID:    ghRepo.ID,
			Owner: ghRepo.Owner.Login,
			Name:  ghRepo.Name,
		}); err != nil {
			return errors.WrapIff(err, "failed to write repository metadata")
		}

		_, _ = fmt.Println("Successfully initialized repository for use with av!")
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&initFlags.Force, "force", false, "force initialization even if metadata already exists")
}
