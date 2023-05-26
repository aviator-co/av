package refmeta

import (
	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
)

func ReadBranchState(repo *git.Repo, branch string, trunk bool) (meta.BranchState, error) {
	if trunk {
		return meta.BranchState{
			Name:  branch,
			Trunk: true,
		}, nil
	}

	head, err := repo.RevParse(&git.RevParse{Rev: "refs/heads/" + branch})
	if err != nil {
		return meta.BranchState{}, errors.WrapIff(
			err,
			"failed to determine HEAD for branch %q",
			branch,
		)
	}
	return meta.BranchState{
		Name: branch,
		Head: head,
	}, nil
}
