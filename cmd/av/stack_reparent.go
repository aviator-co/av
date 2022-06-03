package main

import (
	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/sliceutils"
	"github.com/spf13/cobra"
	"strings"
)

var stackReparentCmd = &cobra.Command{
	Use:   "reparent <new-parent>",
	Short: "change the stack parent of the current branch",
	Long: strings.TrimSpace(`
Change the stack parent of the current branch.

This command can be used to add and/or remove a branch from a stack.
`),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			_ = cmd.Usage()
			return errors.New("expected exactly one argument")
		}
		newParent := args[0]

		repo, err := getRepo()
		if err != nil {
			return err
		}
		branch, err := repo.CurrentBranchName()
		if err != nil {
			return errors.WrapIf(err, "failed to determine current branch")
		}
		diff, err := repo.Diff(&git.DiffOpts{Commit: "HEAD", Quiet: true})
		if err != nil {
			return err
		}
		if !diff.Empty {
			return errors.New("refusing to re-parent: there are un-committed changes")
		}
		defaultBranch, err := repo.DefaultBranch()
		if err != nil {
			return errors.WrapIf(err, "failed to determine default branch")
		}

		// Check that the parent branch actually exists
		if _, err := repo.RevParse(&git.RevParse{Rev: newParent}); err != nil {
			return errors.Errorf("parent branch %q does not exist", newParent)
		}

		branchMeta, _ := meta.ReadBranch(repo, branch)

		// We might need to rebase the branch on top of the new parent. This
		// requires a special rebase command because the "normal" rebase command
		// (without --onto) will consider every commit in the current branch when
		// figuring out what to be played on top of the new parent. So, for example,
		// if we have a stack that looks like B1->B2->B3 with corresponding commits
		// C1->C2->C3, and we want to reparent B3 onto B1, we want to only play C3
		// on top of C1 (and completely ignore C2).
		// If we do `git rebase B1`, Git will try look at all commits which are
		// reachable from C3 but not C1, and play them on top of C1. In particular,
		// it sees that C2 and C3 are reachable from C3, so after the rebase, B3
		// looks like C1->C2->C3, which is wrong.
		// Instead, we need to do `git rebase --onto B1 B2 B3` which says to play
		// the commits that are reachable from B3 but not B2 on top of B1.
		if oldParent := branchMeta.Parent; oldParent != "" {
			// TODO: We might get a rebase conflict during this, which we'll need to
			// 		handle gracefully.
			_, err := repo.Git("rebase", "--onto", newParent, oldParent, branch)
			if err != nil {
				return errors.WrapIff(err, "failed to rebase %q on top of %q", branch, newParent)
			}

			oldParentMeta, ok := meta.ReadBranch(repo, oldParent)
			if ok {
				oldParentMeta.Children = sliceutils.DeleteElement(oldParentMeta.Children, branch)
				if err := meta.WriteBranch(repo, oldParentMeta); err != nil {
					return errors.WrapIff(err, "failed to write branch meta for %q", oldParent)
				}
			}
		}

		if newParent == defaultBranch {
			branchMeta.Parent = ""
		} else {
			branchMeta.Parent = newParent
			parentMeta, _ := meta.ReadBranch(repo, newParent)
			parentMeta.Children = append(parentMeta.Children, branch)
			if err := meta.WriteBranch(repo, parentMeta); err != nil {
				return errors.WrapIff(err, "failed to write branch meta for %q", newParent)
			}
		}
		if err := meta.WriteBranch(repo, branchMeta); err != nil {
			return errors.WrapIff(err, "failed to write branch meta for %q", branch)
		}

		return nil
	},
}
