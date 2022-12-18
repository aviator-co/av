package main

import (
	"context"
	"fmt"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/config"
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
			_, err := meta.ReadRepository(repo)
			if err == nil {
				return errors.New("repository is already initialized for use with av")
			}
		}

		if config.Av.GitHub.Token == "" {
			return errors.New("github token must be set")
		}
		client, err := getClient(config.Av.GitHub.Token)
		if err != nil {
			return err
		}

		// for now we only work with default remote config
		remote, err := repo.DefaultRemote()
		if err != nil {
			return err
		}

		ghRepo, err := client.GetRepositoryBySlug(context.Background(), remote.RepoSlug)
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
