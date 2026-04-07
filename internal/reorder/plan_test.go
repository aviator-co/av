package reorder

import (
	"testing"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/git/gittest"
	"github.com/aviator-co/av/internal/meta"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutosquashPickCmds(t *testing.T) {
	pick := func(commit, comment string) PickCmd {
		return PickCmd{Commit: commit, Comment: comment}
	}

	t.Run("no fixups", func(t *testing.T) {
		picks := []PickCmd{pick("a", "Add feature"), pick("b", "Fix bug")}
		assert.Equal(t, picks, autosquashPickCmds(picks))
	})

	t.Run("fixup placed after target", func(t *testing.T) {
		picks := []PickCmd{
			pick("a", "Add feature"),
			pick("b", "Fix bug"),
			pick("c", "fixup! Add feature"),
		}
		want := []PickCmd{
			pick("a", "Add feature"),
			{Commit: "c", Comment: "fixup! Add feature", Mode: PickModeFixup},
			pick("b", "Fix bug"),
		}
		assert.Equal(t, want, autosquashPickCmds(picks))
	})

	t.Run("squash placed after target", func(t *testing.T) {
		picks := []PickCmd{
			pick("a", "Add feature"),
			pick("b", "Fix bug"),
			pick("c", "squash! Add feature"),
		}
		want := []PickCmd{
			pick("a", "Add feature"),
			{Commit: "c", Comment: "squash! Add feature", Mode: PickModeSquash},
			pick("b", "Fix bug"),
		}
		assert.Equal(t, want, autosquashPickCmds(picks))
	})

	t.Run("multiple fixups for same target preserve order", func(t *testing.T) {
		picks := []PickCmd{
			pick("a", "Add feature"),
			pick("b", "fixup! Add feature"),
			pick("c", "Fix bug"),
			pick("d", "fixup! Add feature"),
		}
		want := []PickCmd{
			pick("a", "Add feature"),
			{Commit: "b", Comment: "fixup! Add feature", Mode: PickModeFixup},
			{Commit: "d", Comment: "fixup! Add feature", Mode: PickModeFixup},
			pick("c", "Fix bug"),
		}
		assert.Equal(t, want, autosquashPickCmds(picks))
	})

	t.Run("fixup with no matching target appended at end", func(t *testing.T) {
		picks := []PickCmd{
			pick("a", "Add feature"),
			pick("b", "fixup! Unknown commit"),
		}
		want := []PickCmd{
			pick("a", "Add feature"),
			{Commit: "b", Comment: "fixup! Unknown commit", Mode: PickModeFixup},
		}
		assert.Equal(t, want, autosquashPickCmds(picks))
	})
}

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
			Name:                     "one",
			Trunk:                    false,
			BranchingPointCommitHash: c1b.String(),
		},
	})
	require.NoError(t, tx.Commit())

	plan, err := CreatePlan(t.Context(), repo.AsAvGitRepo(), db.ReadTx(), "one")
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
