package main

import (
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/sirupsen/logrus"
)

func getRepoInfo() (*git.Repo, *meta.Repository, error) {
	repo, err := getRepo()
	if err != nil {
		return nil, nil, err
	}

	repoMeta, err := meta.ReadRepository(repo)
	if err != nil {
		return nil, nil, err
	}

	logrus.Debugf("loaded repository metadata: %+v", repoMeta)
	return repo, &repoMeta, nil
}
