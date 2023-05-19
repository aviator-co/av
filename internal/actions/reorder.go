package actions

import (
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
)

type ReorderOpts struct {
	Continue bool
	Abort    bool
}

func Reorder(repo *git.Repo, tx meta.WriteTx, opts ReorderOpts) error {
	panic("not implemented")
}
