package stacks

import (
	"bytes"
	"emperror.dev/errors"
	"encoding/json"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/templateutils"
	"github.com/sirupsen/logrus"
	"text/template"
)

type branchMetadata struct {
	// The branch name associated with this stack.
	// Not stored in JSON because the name can always be derived from the name
	// of the git ref.
	Name string `json:"-"`
	// The branch name associated with the parent of the stack (if any).
	Parent string `json:"parent"`
}

type BranchOpts struct {
	Name string
}

// CreateBranch creates a new stack branch based off of the current branch.
func CreateBranch(repo *git.Repo, opts *BranchOpts) error {
	if opts.Name == "" {
		return errors.New("new branch name is required")
	}

	parentBranch, err := repo.CurrentBranchName()
	if err != nil {
		return errors.WrapIff(err, "failed to get current branch name")
	}

	if _, err := repo.RevParse(&git.RevParse{Rev: opts.Name}); err == nil {
		return errors.Errorf("branch %q already exists", opts.Name)
	}

	// There are three scenarios here.
	// Let's suppose we want to get to something that looks like:
	//     main = ... -> X
	//     PR1  =  X -> 1a -> 1b -> 1c
	//     PR2  = 1S -> 2a -> 2b -> 2c
	//     PR3  = 2S -> 3a -> 3b -> 3c
	// [notation]:
	//     * PR{n} is a branch (we use PR1 to denote that it will become the first
	//       PR in the stack, but for these purposes, it's just a branch).
	//     * We say <branch> = <commit> because branches are simply pointers to
	//       commits as far as Git is concerned.
	//     * A commit encapsulates its history (in particular, a commit is
	//       defined by its tree and parent(s)), so a branch is completely
	//       determined by a commit. HOWEVER, for illustrative purposes, the
	//       parents of a commit are sometimes show as `1 -> 2 -> 3` to denote
	//       that 1 is a parent of 2 is a parent of 3.
	// 1. When we create the branch PR1, it's just a vanilla branch. It's the
	//    root of the stack. We do nothing special. It has no stack metadata
	//    because we don't consider it a stacked branch (otherwise even
	//    non-stacked PRs would actually be considered stacks).
	// 2. When we create the branch PR2, we need to determine that it is in fact
	//    a stacked branch. We do this by looking at the metadata of PR1,
	//    realizing that there is none, and so figuring out that it will be the
	//    root of the stack we're about to create (because, again, we only
	//    consider it a stack once it has size >= 2). We write the metadata to
	//    inform us that the parent of PR2 is PR1. Additionally, we do our
	//    clever thing where instead of directly branching off of 1c, we turn
	//    `1a -> 1b -> 1c` into a single squash commit `1S` (whose commit
	//    parent is X since we take the parent of `1a`).
	// 3. When we create the branch PR3, again we look at the stack metadata for
	//    the parent branch PR2. We see that it in fact DOES have stack
	//    metadata, and we need to use this stack metadata to figure out that we
	//    need to construct the base branch here as `X -> 1S -> 2S`.

	// Situation 1: creating a branch directly off of trunk
	if parentBranch == repo.DefaultBranch() {
		// This is the root of the stack.
		// We don't need to do anything special.
		// We just need to create the branch.
		if _, err := repo.CheckoutBranch(&git.CheckoutBranch{
			Name:      opts.Name,
			NewBranch: true,
		}); err != nil {
			logrus.WithError(err).Debugf("failed to checkout branch %q", opts.Name)
			return errors.Errorf(
				"failed to create branch %q (does it already exist?)",
				opts.Name,
			)
		}
		return nil
	}

	// In situations 2 and 3, we need to create a squash commit that
	// incorporates all the changes from previous branch on top of the previous
	// branch's base. Generally, to create branch `PR{n}`, we need:
	//     PR{n} = committree(base(PR{n-1}), tree(PR{n-1}))
	// where base(x) is the commit at the head of the base branch of x and
	// committree(x, y) is the commit created with parent x and tree y (i.e.,
	// a squash commit of y onto x).
	// In terms of the example above, for PR2, we need to create the branch
	//     base(PR2) = committree(base(PR1), tree(PR1))
	//               = committree(X, tree(1c))
	//               = X -> 1S
	// and for PR3:
	//     base(PR3) = committree(base(PR2), tree(PR2))
	//               = committree(1S, tree(2c))
	//               = X -> 1S -> 2S
	parentMeta := readStackMetadata(repo, parentBranch)
	parentHead, err := repo.RevParse(&git.RevParse{Rev: "refs/heads/" + parentBranch})
	if err != nil {
		return errors.WrapIf(err, "failed to determine parent HEAD sha")
	}

	// We determine the parentBase commit differently for scenario 1/2.
	var parentBase string
	if parentMeta == nil {
		// Situation 2: creating the second branch in a stack
		// This amounts to finding the commit where parent branched off of trunk.
		var err error
		parentBase, err = repo.Git("merge-base", repo.DefaultBranch(), "HEAD")
		if err != nil {
			return errors.WrapIff(
				err,
				"failed to determine merge base for commit %s and %s",
				git.ShortSha(parentHead), repo.DefaultBranch(),
			)
		}
		if parentBase == "" {
			return errors.New("merge base is empty")
		}
	} else {
		// Situation 3: creating a branch off of an already established stack.
		parentBaseRefName := "refs/av/stack-base/" + parentBranch
		var err error
		parentBase, err = repo.RevParse(&git.RevParse{Rev: parentBaseRefName})
		if err != nil {
			logrus.WithError(err).Debug("failed to determine parent base commit")
			return errors.Errorf("failed to determine parent base commit: failed to parse git ref %q", parentBaseRefName)
		}
	}

	// Create the squash commit: committree(parentBase, tree(parentHead))
	logrus.Debugf(
		"constructing squash commit: %s..%s",
		git.ShortSha(parentBase),
		git.ShortSha(parentHead),
	)
	message, err := templateutils.String(squashCommitTemplate, squashCommitArgs{
		Branch:       opts.Name,
		ParentBranch: parentBranch,
		ParentHead:   parentHead,
	})
	if err != nil {
		return errors.WrapIf(err, "failed to render commit message")
	}
	squashCommit, err := repo.Git(
		"commit-tree",
		parentHead+"^{tree}", "-p", parentBase,
		"-m", message,
	)

	// Save the squash commit as a new ref.
	// TODO:
	//     We'll need to push this ref as a branch to GitHub (nb: this ref is
	//     *not* a branch and we can't open a PR for it as-is), but ideally we
	//     can do that without creating a branch in the local repo (which would
	//     pollute `git branch` output).
	baseRefName := "refs/av/stack-base/" + opts.Name
	if err := repo.UpdateRef(&git.UpdateRef{Ref: baseRefName, New: squashCommit}); err != nil {
		return err
	}
	logrus.
		WithFields(logrus.Fields{"ref": baseRefName, "oid": squashCommit}).
		Debug("created stack-base ref")

	// Create the new branch...
	headRefName := "refs/heads/" + opts.Name
	if err := repo.UpdateRef(&git.UpdateRef{
		Ref: headRefName,
		New: squashCommit,
		Old: git.Missing,
	}); err != nil {
		return errors.WrapIff(err, "cannot create branch %q", opts.Name)
	}
	// ...and check it out.
	// Note: we need to use the branch name (e.g., feature-1) rather than the
	// "fully-qualified" ref name (e.g., refs/heads/feature-1) because the
	// latter tells git to enter the "detached HEAD" state.
	if _, err := repo.CheckoutBranch(&git.CheckoutBranch{
		Name: opts.Name,
	}); err != nil {
		return errors.WrapIff(err, "failed to checkout new branch %s", opts.Name)
	}

	// Finally, write the metadata.
	if err := writeStackMetadata(repo, &branchMetadata{
		Name:   opts.Name,
		Parent: parentBranch,
	}); err != nil {
		return errors.WrapIff(err, "failed to write av internal metadata for branch %q", opts.Name)
	}
	return nil
}

func stackMetadataRefName(branchName string) string {
	return "refs/av/stack-metadata/" + branchName
}

func writeStackMetadata(repo *git.Repo, s *branchMetadata) error {
	refName := stackMetadataRefName(s.Name)
	content, err := json.Marshal(s)
	if err != nil {
		return errors.Wrap(err, "failed to marshal stack metadata")
	}
	objectId, err := repo.GitStdin(
		[]string{"hash-object", "-w", "--stdin"},
		bytes.NewReader(content),
	)
	if err != nil {
		return errors.Wrap(err, "failed to store stack metadata in git")
	}
	if err := repo.UpdateRef(&git.UpdateRef{Ref: refName, New: objectId}); err != nil {
		return err
	}
	logrus.
		WithFields(logrus.Fields{"ref": refName, "sha": git.ShortSha(objectId)}).
		Debug("created stack ref")
	return nil
}

// readStackMetadata looks up the stack metadata for the given branch name.
// It returns the branchMetadata object or nil if it does not exist.
func readStackMetadata(repo *git.Repo, branchName string) *branchMetadata {
	refName := stackMetadataRefName(branchName)
	blob, err := repo.Git("cat-file", "blob", refName)

	// Just assume that any error here means that the metadata ref doesn't exist
	// (there's no easy way to distinguish between that and an actual Git error)
	if err != nil {
		return nil
	}

	var branch branchMetadata
	if err := json.Unmarshal([]byte(blob), &branch); err != nil {
		logrus.WithError(err).WithField("ref", refName).Error("corrupt stack metadata, deleting...")
		_ = repo.UpdateRef(&git.UpdateRef{Ref: refName, New: git.Missing})
		return nil
	}

	return &branch
}

var squashCommitTemplate = template.Must(template.New("squash commit message").Parse(
	`Synthetic squash base commit for {{.Branch}}.

av automatically creates this commit by squashing the changes from the parent
branch ({{.ParentBranch}}). This is required to work with squash commits on
GitHub, but this commit will ultimately not be added to your repository base
branch.

If changes have been made to the parent branch ({{.ParentBranch}}),
from the head of branch {{.Branch}}, run
    av stack sync
to synchronize changes across the stack, or run
    av stack sync --current
to only synchronize changes from the previous branch(es).

## av internal metadata, do not edit
{{- /*
TODO(travis):
    This might not actually be necessary because we can get the parent branch
    from the metadata ref that we store, and we can determine if we need to
    rebase based on whether-or-not the tree of the parent branch is the same as
    the tree of this commit.
*/ }}
av-stack-parent-branch: {{.ParentBranch}}
av-stack-parent-head:   {{.ParentHead}}
`))

type squashCommitArgs struct {
	Branch       string
	ParentBranch string
	ParentHead   string
}
