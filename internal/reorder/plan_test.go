package reorder

import (
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCreatePlan(t *testing.T) {
	repo := gittest.NewTempRepo(t)

	db, err := jsonfiledb.OpenRepo(repo)
	require.NoError(t, err)

	initialCommit, err := repo.RevParse(&git.RevParse{Rev: "HEAD"})
	require.NoError(t, err)
	_ = initialCommit

	_, err = repo.CheckoutBranch(&git.CheckoutBranch{Name: "one", NewBranch: true})
	require.NoError(t, err)
	c1a := gittest.CommitFile(t, repo, "file", []byte("hello\n"))
	c1b := gittest.CommitFile(t, repo, "file", []byte("hello\nworld\n"))
	_ = c1a
	_ = c1b

	_, err = repo.CheckoutBranch(&git.CheckoutBranch{Name: "two", NewBranch: true})
	require.NoError(t, err)
	c2a := gittest.CommitFile(t, repo, "fichier", []byte("bonjour\n"))
	c2b := gittest.CommitFile(t, repo, "fichier", []byte("bonjour\nle monde\n"))
	_ = c2a
	_ = c2b

	tx := db.WriteTx()
	tx.SetBranch(meta.Branch{
		Name: "one",
		Parent: meta.BranchState{
			Name:  "main",
			Trunk: true,
		},
	})
	tx.SetBranch(meta.Branch{
		Name: "two",
		Parent: meta.BranchState{
			Name:  "one",
			Trunk: false,
			Head:  c1b,
		},
	})
	require.NoError(t, tx.Commit())

	plan, err := CreatePlan(repo, db.ReadTx(), "one")
	require.NoError(t, err)
	for _, cmd := range plan {
		t.Log(cmd.String())
	}

	want := []Cmd{
		StackBranchCmd{Name: "one", Trunk: "main@" + initialCommit},
		PickCmd{Commit: c1a},
		PickCmd{Commit: c1b},
		StackBranchCmd{Name: "two", Parent: "one"},
		PickCmd{Commit: c2a},
		PickCmd{Commit: c2b},
	}
	// This is a little bit fragile but :shrug:
	for i, cmd := range plan {
		if sb, ok := cmd.(StackBranchCmd); ok {
			wantSb := want[i].(StackBranchCmd)
			require.Equal(t, wantSb.Name, sb.Name)
		} else if p, ok := cmd.(PickCmd); ok {
			wantP := want[i].(PickCmd)
			require.Equal(t, git.ShortSha(wantP.Commit), p.Commit)
		} else {
			t.Fatalf("unexpected command type: %T", cmd)
		}
	}
}
