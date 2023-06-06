package actions

import (
	"fmt"
	"os"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/sirupsen/logrus"
)

type ReparentOpts struct {
	// The name of the branch to re-parent.
	Branch string
	// The new parent branch to re-parent the branch to.
	NewParent string
	// If true, consider the NewParent a trunk branch.
	NewParentTrunk bool
}

type ReparentResult struct {
	Success bool
	Hint    string
}

// Reparent changes the parent branch of a stacked branch (performing a rebase
// if necessary).
func Reparent(
	repo *git.Repo,
	tx meta.WriteTx,
	opts ReparentOpts,
) (*ReparentResult, error) {
	branchMeta, exist := tx.Branch(opts.Branch)
	if !exist {
		_, _ = fmt.Fprint(
			os.Stderr,
			"  - Adopting a branch ",
			colors.UserInput(opts.Branch),
			" to Av",
			colors.UserInput(opts.NewParent),
			"\n",
		)
		branchMeta.Parent.Name = opts.NewParent
		branchMeta.Parent.Trunk = opts.NewParentTrunk
		if !branchMeta.Parent.Trunk {
			head, err := repo.RevParse(&git.RevParse{Rev: opts.NewParent})
			if err != nil {
				return nil, errors.WrapIff(err, "failed to get HEAD of %q", opts.NewParent)
			}
			branchMeta.Parent.Head = head
		}
		tx.SetBranch(branchMeta)
	}

	_, _ = fmt.Fprint(os.Stderr,
		"  - Re-parenting branch ", colors.UserInput(opts.Branch),
		" onto ", colors.UserInput(opts.NewParent),
		"\n",
	)

	diff, err := repo.Diff(&git.DiffOpts{Specifiers: []string{"HEAD"}, Quiet: true})
	if err != nil {
		return nil, err
	}
	if !diff.Empty {
		_, _ = fmt.Fprint(os.Stderr,
			colors.Failure("      - ERROR:"),
			" refusing to re-parent ", colors.UserInput(opts.Branch),
			" onto ", colors.UserInput(opts.NewParent),
			": the working tree has uncommitted changes\n",
		)
		_, _ = colors.TroubleshootingC.Fprint(os.Stderr,
			"      - HINT: commit, stash, or reset your uncommitted changes first\n",
		)
		return nil, errors.New("refusing to re-parent: there are un-committed changes")
	}

	// Check that the parent branch actually exists
	parentBranch := opts.NewParent
	if opts.NewParentTrunk {
		parentBranch = "remotes/origin/" + opts.NewParent
	}
	parentSha, err := repo.RevParse(&git.RevParse{Rev: parentBranch})
	if err != nil {
		_, _ = fmt.Fprint(os.Stderr,
			colors.Failure("      - ERROR:"),
			"cannot re-parent branch ", colors.UserInput(opts.Branch),
			": new parent branch ", colors.UserInput(parentBranch),
			" does not exist\n",
		)
		return nil, errors.Errorf("parent branch %q does not exist", parentBranch)
	}

	upstream := branchMeta.Parent.Name
	if branchMeta.Parent.Trunk {
		upstream = "remotes/origin/" + branchMeta.Parent.Name
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
	logrus.WithFields(logrus.Fields{
		"branch":      opts.Branch,
		"onto_branch": parentBranch,
		"onto_head":   parentSha,
		"upstream":    upstream,
	}).Debug("rebasing branch")
	output, err := repo.Rebase(git.RebaseOpts{
		Onto:     parentSha,
		Upstream: upstream,
		Branch:   opts.Branch,
	})
	if err != nil {
		return nil, errors.WrapIff(err, "failed to run git rebase")
	}

	return handleReparentRebaseOutput(repo, tx, opts, output)
}

func ReparentSkipContinue(
	repo *git.Repo,
	tx meta.WriteTx,
	opts ReparentOpts,
	skip bool,
) (*ReparentResult, error) {
	var rebaseOpts git.RebaseOpts
	if skip {
		rebaseOpts.Skip = true
	} else {
		rebaseOpts.Continue = true
	}
	output, err := repo.Rebase(rebaseOpts)
	if err != nil {
		return nil, err
	}

	if output.ExitCode != 0 && strings.Contains(string(output.Stderr), "no rebase in progress") {
		// If there's no rebase, assume the user did `git rebase --continue/--skip` manually.
		// TODO: we could try to detect if the user `git rebase --abort`-ed here
		_, _ = fmt.Fprint(os.Stderr,
			"    - ", colors.Failure("WARNING: "),
			"no rebase in progress -- assuming rebase was completed (not aborted)",
			"\n",
		)
		if err := reparentWriteMetadata(repo, tx, opts); err != nil {
			return nil, err
		}
		return &ReparentResult{Success: true}, nil
	}
	return handleReparentRebaseOutput(repo, tx, opts, output)
}

func reparentWriteMetadata(
	repo *git.Repo,
	tx meta.WriteTx,
	opts ReparentOpts,
) error {
	branch, _ := tx.Branch(opts.Branch)

	var parentHead string
	if !opts.NewParentTrunk {
		var err error
		parentHead, err = repo.RevParse(&git.RevParse{Rev: opts.NewParent})
		if err != nil {
			return errors.WrapIff(err, "failed to read head commit of %q", opts.NewParent)
		}
	}

	branch.Parent = meta.BranchState{
		Name:  opts.NewParent,
		Trunk: opts.NewParentTrunk,
		Head:  parentHead,
	}
	tx.SetBranch(branch)

	return nil
}

func handleReparentRebaseOutput(
	repo *git.Repo,
	tx meta.WriteTx,
	opts ReparentOpts,
	output *git.Output,
) (*ReparentResult, error) {
	if output.ExitCode != 0 {
		_, _ = fmt.Fprint(os.Stderr,
			colors.Failure("      - ERROR:"),
			" conflict while rebasing branch ", colors.UserInput(opts.Branch),
			" onto ", colors.UserInput(opts.NewParent),
			"\n",
		)
		return &ReparentResult{Success: false, Hint: string(output.Stderr)}, nil
	}

	if err := reparentWriteMetadata(repo, tx, opts); err != nil {
		return nil, err
	}
	return &ReparentResult{Success: true}, nil
}
