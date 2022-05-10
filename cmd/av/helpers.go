package main

import (
	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/sirupsen/logrus"
)

func getRepoInfo() (*git.Repo, *meta.Repository, error) {
	repo, err := getRepo()
	if err != nil {
		return nil, nil, err
	}

	repoMeta, ok := meta.GetRepository(repo)
	if !ok {
		return nil, nil, errors.New("this repository is not initialized for us with av: please run `av init`")
	}

	logrus.Debugf("loaded repository metadata: %+v", repoMeta)
	return repo, &repoMeta, nil
}
