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

func TestAutosquashCmds(t *testing.T) {
	pick := func(commit, comment string) Cmd {
		return PickCmd{Commit: commit, Comment: comment}
	}
	branch := func(name string) Cmd {
		return StackBranchCmd{Name: name}
	}

	t.Run("no fixups", func(t *testing.T) {
		cmds := []Cmd{pick("a", "Add feature"), pick("b", "Fix bug")}
		assert.Equal(t, cmds, autosquashCmds(cmds))
	})

	t.Run("fixup placed after target", func(t *testing.T) {
		cmds := []Cmd{
			pick("a", "Add feature"),
			pick("b", "Fix bug"),
			pick("c", "fixup! Add feature"),
		}
		want := []Cmd{
			pick("a", "Add feature"),
			PickCmd{Commit: "c", Comment: "fixup! Add feature", Mode: PickModeFixup},
			pick("b", "Fix bug"),
		}
		assert.Equal(t, want, autosquashCmds(cmds))
	})

	t.Run("squash placed after target", func(t *testing.T) {
		cmds := []Cmd{
			pick("a", "Add feature"),
			pick("b", "Fix bug"),
			pick("c", "squash! Add feature"),
		}
		want := []Cmd{
			pick("a", "Add feature"),
			PickCmd{Commit: "c", Comment: "squash! Add feature", Mode: PickModeSquash},
			pick("b", "Fix bug"),
		}
		assert.Equal(t, want, autosquashCmds(cmds))
	})

	t.Run("multiple fixups for same target preserve order", func(t *testing.T) {
		cmds := []Cmd{
			pick("a", "Add feature"),
			pick("b", "fixup! Add feature"),
			pick("c", "Fix bug"),
			pick("d", "fixup! Add feature"),
		}
		want := []Cmd{
			pick("a", "Add feature"),
			PickCmd{Commit: "b", Comment: "fixup! Add feature", Mode: PickModeFixup},
			PickCmd{Commit: "d", Comment: "fixup! Add feature", Mode: PickModeFixup},
			pick("c", "Fix bug"),
		}
		assert.Equal(t, want, autosquashCmds(cmds))
	})

	t.Run("fixup with target absent from entire stack gets warning", func(t *testing.T) {
		cmds := []Cmd{
			pick("a", "Add feature"),
			pick("b", "fixup! Unknown commit"),
		}
		want := []Cmd{
			pick("a", "Add feature"),
			PickCmd{
				Commit:  "b",
				Comment: `WARNING: target commit "Unknown commit" not found in the stack`,
				Mode:    PickModePick,
			},
		}
		assert.Equal(t, want, autosquashCmds(cmds))
	})

	t.Run("fixup targeting commit in another branch is placed after target", func(t *testing.T) {
		// "fixup! foo" lives in branch-two but targets "foo" in branch-one.
		// It should be moved to branch-one's section, right after "foo".
		cmds := []Cmd{
			branch("branch-one"),
			pick("a", "foo"),
			pick("b", "bar"),
			branch("branch-two"),
			pick("c", "baz"),
			pick("d", "fixup! foo"),
		}
		want := []Cmd{
			branch("branch-one"),
			pick("a", "foo"),
			PickCmd{Commit: "d", Comment: "fixup! foo", Mode: PickModeFixup},
			pick("b", "bar"),
			branch("branch-two"),
			pick("c", "baz"),
		}
		assert.Equal(t, want, autosquashCmds(cmds))
	})

	t.Run("fixup preceding its target is placed after target", func(t *testing.T) {
		// fixup! foo appears before foo — the implementation finds the *last*
		// non-fixup commit matching the target title and places the fixup after
		// it. Since foo appears after the fixup, the fixup should follow foo.
		cmds := []Cmd{
			pick("a", "fixup! foo"),
			pick("b", "foo"),
			pick("c", "bar"),
		}
		want := []Cmd{
			pick("b", "foo"),
			PickCmd{Commit: "a", Comment: "fixup! foo", Mode: PickModeFixup},
			pick("c", "bar"),
		}
		assert.Equal(t, want, autosquashCmds(cmds))
	})

	t.Run("mixed fixup and squash on same target preserve relative order", func(t *testing.T) {
		// Both fixup! foo and squash! foo target "foo". They should both be
		// placed directly after "foo" with correct modes, and their relative
		// order (fixup first, squash second, as they appear in the original
		// list) must be preserved.
		cmds := []Cmd{
			pick("a", "foo"),
			pick("b", "fixup! foo"),
			pick("c", "bar"),
			pick("d", "squash! foo"),
		}
		want := []Cmd{
			pick("a", "foo"),
			PickCmd{Commit: "b", Comment: "fixup! foo", Mode: PickModeFixup},
			PickCmd{Commit: "d", Comment: "squash! foo", Mode: PickModeSquash},
			pick("c", "bar"),
		}
		assert.Equal(t, want, autosquashCmds(cmds))
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
