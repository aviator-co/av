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
	// If true, require the upstream tracking information to already be set
	// (otherwise, don't push).
	SkipIfUpstreamNotSet bool
	// If true, skip pushing the branch if the upstream commit is the same as
	// the local HEAD commit. The caller should probably call `git fetch` before
	// running this to make sure remote tracking information is up-to-date.
	SkipIfUpstreamMatches bool
}

// Push pushes the current branch to the Git remote.
// It does not check out the given branch.
func Push(repo *git.Repo, opts PushOpts) error {
	currentBranch, err := repo.CurrentBranchName()
	if err != nil {
		return errors.WrapIff(err, "failed to determine current branch")
	}

	if opts.SkipIfUpstreamNotSet || opts.SkipIfUpstreamMatches {
		upstream, err := repo.RevParse(&git.RevParse{Rev: "@{upstream}"})
		if opts.SkipIfUpstreamMatches && git.StderrMatches(err, "no upstream") {
			_, _ = fmt.Fprint(os.Stderr,
				"  - not pushing branch ", colors.UserInput(currentBranch),
				" (no upstream is set)\n",
				colors.Troubleshooting("      - HINT: create a pr with "),
				colors.CliCmd("av pr create"),
				colors.Troubleshooting(" to automatically push the branch to GitHub and create a pull request\n"),
			)
			return nil
		} else if err != nil {
			// This usually indicates a detached HEAD state
			return errors.WrapIff(err, "failed to determine upstream tracking information for branch %q", currentBranch)
		}

		head, err := repo.RevParse(&git.RevParse{Rev: "HEAD"})
		if err != nil {
			return errors.WrapIff(err, "failed to determine branch HEAD for branch %q", currentBranch)
		}
		if opts.SkipIfUpstreamMatches && upstream == head {
			_, _ = fmt.Fprint(os.Stderr,
				"  - not pushing branch ", colors.UserInput(currentBranch),
				" (upstream is already up-to-date)\n",
			)
			return nil
		}
	}

	_, _ = fmt.Fprint(os.Stderr,
		"  - pushing ", colors.UserInput(currentBranch), "... ",
	)
	pushArgs := []string{"push", "--set-upstream"}
	switch opts.Force {
	case NoForce:
		// pass
	case ForceWithLease:
		pushArgs = append(pushArgs, "--force-with-lease")
	case ForcePush:
		pushArgs = append(pushArgs, "--force")
	}
	res, err := repo.Run(&git.RunOpts{
		Args: pushArgs,
	})
	if err != nil {
		_, _ = fmt.Fprint(os.Stderr,
			colors.Failure("error: ", err.Error()),
			"\n",
		)
		return errors.WrapIff(err, "failed to push branch %q", currentBranch)
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
		return errors.WrapIff(err, "failed to push branch %q", currentBranch)
	}
	_, _ = fmt.Fprint(os.Stderr,
		colors.Success("okay"), "\n",
	)
	return nil
}
