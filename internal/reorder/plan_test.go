package reorder

import (
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"
)

func TestCreatePlan(t *testing.T) {
	repo := gittest.NewTempRepo(t)

	db := repo.OpenDB(t)

	initialCommit := repo.GetCommitAtRef(t, plumbing.HEAD)

	repo.CreateRef(t, plumbing.NewBranchReferenceName("one"))
	repo.CheckoutBranch(t, plumbing.NewBranchReferenceName("one"))
	c1a := repo.CommitFile(t, "file", "hello\n")
	c1b := repo.CommitFile(t, "file", "hello\nworld\n")

	repo.CreateRef(t, plumbing.NewBranchReferenceName("two"))
	repo.CheckoutBranch(t, plumbing.NewBranchReferenceName("two"))
	c2a := repo.CommitFile(t, "fichier", "bonjour\n")
	c2b := repo.CommitFile(t, "fichier", "bonjour\nle monde\n")

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
			Head:  c1b.String(),
		},
	})
	require.NoError(t, tx.Commit())

	plan, err := CreatePlan(repo.AsAvGitRepo(), db.ReadTx(), "one")
	require.NoError(t, err)
	for _, cmd := range plan {
		t.Log(cmd.String())
	}

	want := []Cmd{
		StackBranchCmd{Name: "one", Trunk: "main@" + initialCommit.String()},
		PickCmd{Commit: c1a.String()},
		PickCmd{Commit: c1b.String()},
		StackBranchCmd{Name: "two", Parent: "one"},
		PickCmd{Commit: c2a.String()},
		PickCmd{Commit: c2b.String()},
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
