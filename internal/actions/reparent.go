package actions

import (
	"emperror.dev/errors"
	"fmt"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/sliceutils"
	"os"
	"strings"
)

type ReparentResult struct {
	Success bool
	Hint    string
}

// Reparent changes the parent branch of a stacked branch (performing a rebase
// if necessary).
func Reparent(repo *git.Repo, branch string, newParent string) (*ReparentResult, error) {
	_, _ = fmt.Fprint(os.Stderr,
		"  - Re-parenting branch ", colors.UserInput(branch),
		" onto ", colors.UserInput(newParent),
		"\n",
	)

	diff, err := repo.Diff(&git.DiffOpts{Commit: "HEAD", Quiet: true})
	if err != nil {
		return nil, err
	}
	if !diff.Empty {
		_, _ = fmt.Fprint(os.Stderr,
			colors.Failure("      - ERROR:"),
			" refusing to re-parent ", colors.UserInput(branch),
			" onto ", colors.UserInput(newParent),
			": the working tree has uncommitted changes\n",
		)
		_, _ = colors.TroubleshootingC.Fprint(os.Stderr,
			"      - HINT: commit, stash, or reset your uncommitted changes first\n",
		)
		return nil, errors.New("refusing to re-parent: there are un-committed changes")
	}

	// Check that the parent branch actually exists
	if _, err := repo.RevParse(&git.RevParse{Rev: newParent}); err != nil {
		_, _ = fmt.Fprint(os.Stderr,
			colors.Failure("      - ERROR:"),
			"cannot re-parent branch ", colors.UserInput(branch),
			": new parent branch ", colors.UserInput(newParent),
			" does not exist\n",
		)
		return nil, errors.Errorf("parent branch %q does not exist", newParent)
	}

	branchMeta, _ := meta.ReadBranch(repo, branch)
	upstream := branchMeta.Parent
	if upstream == "" {
		var err error
		upstream, err = repo.DefaultBranch()
		if err != nil {
			return nil, errors.Wrap(err, "failed to determine repo default branch")
		}
	}

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
	output, err := repo.Rebase(git.RebaseOpts{
		Onto:     newParent,
		Upstream: upstream,
		Branch:   branch,
	})
	if err != nil {
		return nil, errors.WrapIff(err, "failed to run git rebase")
	}

	return handleReparentRebaseOutput(repo, branch, newParent, output)
}

func ReparentContinue(repo *git.Repo, branch string, newParent string) (*ReparentResult, error) {
	output, err := repo.Rebase(git.RebaseOpts{
		Continue: true,
	})
	if err != nil {
		return nil, err
	}

	if output.ExitCode != 0 && strings.Contains(string(output.Stderr), "no rebase in progress") {
		// If there's no rebase, assume the user did `git rebase --continue` manually.
		// TODO: we could try to detect if the user `git rebase --abort`-ed here
		_, _ = fmt.Fprint(os.Stderr,
			"    - ", colors.Failure("WARNING: "),
			"no rebase in progress -- assuming rebase was completed (not aborted)",
			"\n",
		)
		if err := reparentWriteMetadata(repo, branch, newParent); err != nil {
			return nil, err
		}
		return &ReparentResult{Success: true}, nil
	}
	return handleReparentRebaseOutput(repo, branch, newParent, output)
}

func reparentWriteMetadata(repo *git.Repo, branch string, newParent string) error {
	defaultBranch, err := repo.DefaultBranch()
	if err != nil {
		return errors.WrapIf(err, "failed to determine default branch")
	}

	branchMeta, _ := meta.ReadBranch(repo, branch)
	oldParent := branchMeta.Parent

	if newParent == defaultBranch {
		branchMeta.Parent = ""
	} else {
		branchMeta.Parent = newParent
	}
	if err := meta.WriteBranch(repo, branchMeta); err != nil {
		return errors.WrapIff(err, "failed to write branch meta for %q", branch)
	}

	if oldParentMeta, ok := meta.ReadBranch(repo, oldParent); ok {
		oldParentMeta.Children = sliceutils.DeleteElement(oldParentMeta.Children, branch)
		if err := meta.WriteBranch(repo, oldParentMeta); err != nil {
			return errors.WrapIff(err, "failed to write branch meta for %q", oldParent)
		}
	}

	if newParent != defaultBranch {
		newParentMeta, _ := meta.ReadBranch(repo, newParent)
		newParentMeta.Children = append(newParentMeta.Children, branch)
		if err := meta.WriteBranch(repo, newParentMeta); err != nil {
			return errors.WrapIff(err, "failed to write branch meta for %q", newParent)
		}
	}

	return nil
}

func handleReparentRebaseOutput(repo *git.Repo, branch string, newParent string, output *git.Output) (*ReparentResult, error) {
	if output.ExitCode != 0 {
		_, _ = fmt.Fprint(os.Stderr,
			colors.Failure("      - ERROR:"),
			" conflict while rebasing branch ", colors.UserInput(branch),
			" onto ", colors.UserInput(newParent),
			"\n",
		)
		return &ReparentResult{Success: false, Hint: string(output.Stderr)}, nil
	}

	if err := reparentWriteMetadata(repo, branch, newParent); err != nil {
		return nil, err
	}
	return &ReparentResult{Success: true}, nil
}
