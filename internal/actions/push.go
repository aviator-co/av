package actions

import (
	"fmt"
	"os"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/kr/text"
	"github.com/sirupsen/logrus"
)

type ForceOpt int

const (
	NoForce ForceOpt = iota
	// ForceWithLease indicates that the push should use the --force-with-lease
	// option which instructs Git that it should only force push to the remote
	// branch if its current HEAD matches what we think it should be.
	ForceWithLease ForceOpt = iota
	ForcePush      ForceOpt = iota
)

type PushOpts struct {
	Force ForceOpt
	// If true, require the corresponding branch exist on the remote (otherwise, don't push).
	SkipIfRemoteBranchNotExist bool
	// If true, skip pushing the branch if the corresponding branch on the remote points to the
	// same commit as the local HEAD commit. The caller should probably call `git fetch` before
	// running this to make sure remote tracking information is up-to-date.
	SkipIfRemoteBranchIsUpToDate bool
}

// Push pushes the given branch to the Git origin.
func Push(repo *git.Repo, branchName string, opts PushOpts) error {
	if opts.SkipIfRemoteBranchNotExist || opts.SkipIfRemoteBranchIsUpToDate {
		// NOTE: This remote branch pattern is configurable with the fetch spec. This code
		// assumes that the user won't change the fetch spec from the default. Technically,
		// this must be generated from the fetch spec.
		remoteBranch := "refs/remotes/origin/" + branchName
		remoteBranchCommit, err := repo.RevParse(&git.RevParse{Rev: remoteBranch})
		if err != nil {
			return errors.WrapIff(err, "corresponding remote branch %q doesn't exist", remoteBranch)
		}

		head, err := repo.RevParse(&git.RevParse{Rev: branchName})
		if err != nil {
			return errors.WrapIff(
				err,
				"failed to determine HEAD for branch %q",
				branchName,
			)
		}
		logrus.WithFields(logrus.Fields{
			"remote_branch": remoteBranch,
			"remote_head":   remoteBranchCommit,
			"local_head":    head,
		}).Debug("checking if remote branch is up-to-date")
		if opts.SkipIfRemoteBranchIsUpToDate && remoteBranchCommit == head {
			_, _ = fmt.Fprint(os.Stderr,
				"  - not pushing branch ", colors.UserInput(branchName),
				" (upstream is already up-to-date)\n",
			)
			return nil
		}
	}

	_, _ = fmt.Fprint(os.Stderr,
		"  - pushing ", colors.UserInput(branchName), "... ",
	)
	pushArgs := []string{"push"}
	switch opts.Force {
	case NoForce:
		// pass
	case ForceWithLease:
		pushArgs = append(pushArgs, "--force-with-lease")
	case ForcePush:
		pushArgs = append(pushArgs, "--force")
	}
	pushArgs = append(pushArgs, "origin", branchName)
	res, err := repo.Run(&git.RunOpts{
		Args: pushArgs,
	})
	if err != nil {
		_, _ = fmt.Fprint(os.Stderr,
			colors.Failure("error: ", err.Error()),
			"\n",
		)
		return errors.WrapIff(err, "failed to push branch %q", branchName)
	}
	if res.ExitCode != 0 {
		_, _ = fmt.Fprint(os.Stderr,
			colors.Failure("failed to push"),
			"\n",
		)
		logrus.WithFields(logrus.Fields{
			"stdout": string(res.Stdout),
			"stderr": string(res.Stderr),
		}).Debug("git push failed")
		if strings.Contains(string(res.Stderr), "stale info") {
			_, _ = colors.TroubleshootingC.Fprint(os.Stderr,
				"      - the remote branch seems to have diverged (were new commits pushed to\n",
				"        it without using av?); to fix this, confirm that the remote branch is\n",
				"        as expected and then force-push this branch\n",
			)
		} else {
			_, _ = colors.TroubleshootingC.Fprint(os.Stderr,
				"      - git output:\n",
				text.Indent(string(res.Stderr), "        "),
				"\n",
			)
		}
		return errors.WrapIff(err, "failed to push branch %q", branchName)
	}
	if err := repo.BranchSetConfig(branchName, "av-pushed-remote", "origin"); err != nil {
		return err
	}
	if err := repo.BranchSetConfig(branchName, "av-pushed-ref", fmt.Sprintf("refs/heads/%s", branchName)); err != nil {
		return err
	}
	_, _ = fmt.Fprint(os.Stderr,
		colors.Success("okay"), "\n",
	)
	return nil
}
