package git

import (
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

type RebaseOpts struct {
	// Required (unless Continue is true)
	// The upstream branch to rebase onto.
	Upstream string
	// Optional (mutually exclusive with all other options)
	// If set, continue a rebase (all other options are ignored).
	Continue bool
	// Optional (mutually exclusive with all other options)
	Abort bool
	// Optional (mutually exclusive with all other options)
	Skip bool
	// Optional
	// If set, use `git rebase --onto <upstream> ...`
	Onto string
	// Optional
	// If set, this is the branch that will be rebased; otherwise, the current
	// branch is rebased.
	Branch string
}

func (r *Repo) Rebase(opts RebaseOpts) (*Output, error) {
	// TODO: probably move the parseRebaseOutput logic in sync to here

	args := []string{"rebase"}
	if opts.Continue {
		return r.Run(&RunOpts{
			Args: []string{"rebase", "--continue"},
			// `git rebase --continue` will open an editor to allow the user
			// to edit the commit message, which we don't want here. Instead, we
			// specify `true` here (which is a command that does nothing and
			// simply exits 0) to disable the editor.
			Env: []string{"GIT_EDITOR=true"},
		})
	} else if opts.Abort {
		return r.Run(&RunOpts{
			Args: []string{"rebase", "--abort"},
		})
	} else if opts.Skip {
		return r.Run(&RunOpts{
			Args: []string{"rebase", "--skip"},
		})
	}
	if opts.Onto != "" {
		args = append(args, "--onto", opts.Onto)
	}
	args = append(args, opts.Upstream)
	if opts.Branch != "" {
		args = append(args, opts.Branch)
	}

	return r.Run(&RunOpts{Args: args})
}

// RebaseParse runs a `git rebase` and parses the output into a RebaseResult.
func (r *Repo) RebaseParse(opts RebaseOpts) (*RebaseResult, error) {
	out, err := r.Rebase(opts)
	if err != nil {
		return nil, err
	}
	return parseRebaseResult(opts, out)
}

type RebaseStatus int

const (
	RebaseAlreadyUpToDate RebaseStatus = iota
	RebaseUpdated         RebaseStatus = iota
	RebaseConflict        RebaseStatus = iota
	RebaseNotInProgress   RebaseStatus = iota
	// RebaseAborted indicates that an in-progress rebase was aborted.
	// Only returned if Rebase was called with Abort: true.
	RebaseAborted RebaseStatus = iota
)

type RebaseResult struct {
	Status RebaseStatus
	Hint   string
	// The "headline" of the error message (if any)
	ErrorHeadline string
}

var carriageReturnRegex = regexp.MustCompile(`^.+\r`)
var hintRegex = regexp.MustCompile(`(?m)^hint:.+$\n?`)
var errorMatchRegex = regexp.MustCompile(`(?m)^error: (.+)$`)

// normalizeRebaseHint normalizes the output (stderr) from running a
// `git rebase` command. We do two things:
//  1. Remove all text that comes before a carriage return on each line (this
//     emulates what the terminal does). This is necessary since Git will print
//     "Rebasing (1/1)" on the start of the rebase and then if it errors, print
//     "\r" (to erase the current text on the line), and then print the error
//     text after.
//  3. Remove the "hint:" lines since they usually include instructions to run
//     the `git rebase --continue` command which is usually not what we want to
//     tell users to do with av.
func normalizeRebaseHint(stderr []byte) string {
	res := string(stderr)
	res = carriageReturnRegex.ReplaceAllString(res, "")
	res = hintRegex.ReplaceAllString(res, "")
	res = strings.ReplaceAll(res, "git rebase", "av stack sync")
	return res
}

func parseRebaseResult(opts RebaseOpts, out *Output) (*RebaseResult, error) {
	stdout := string(out.Stdout)
	stderr := string(out.Stderr)

	if out.ExitCode == 0 {
		if strings.Contains(stderr, "Successfully rebased") {
			return &RebaseResult{Status: RebaseUpdated}, nil
		}
		// For some reason, only this message seems to be printed to stdout
		// (everything else goes to stderr)
		if strings.Contains(stdout, "is up to date") {
			return &RebaseResult{Status: RebaseAlreadyUpToDate}, nil
		}

		if opts.Abort {
			return &RebaseResult{Status: RebaseAborted}, nil
		}

		logrus.WithFields(logrus.Fields{
			"stderr": stderr,
			"stdout": string(out.Stdout),
		}).Warnf("unexpected output from git rebase with exit code 0 (assuming rebase was successful)")
		return &RebaseResult{Status: RebaseUpdated}, nil
	}

	var status RebaseStatus
	lowerStderr := strings.ToLower(stderr)
	switch {
	case strings.Contains(lowerStderr, "no rebase in progress"):
		status = RebaseNotInProgress
	case strings.Contains(lowerStderr, "could not apply"):
		status = RebaseConflict
	default:
		logrus.WithField("exit_code", out.ExitCode).
			Warn("unexpected output from git rebase with non-zero exit code (assuming rebase had conflicts): ", stderr)
		return &RebaseResult{
			Status: RebaseConflict,
			Hint:   stderr,
		}, nil
	}

	hint := normalizeRebaseHint(out.Stderr)
	headline := ""
	errorMatches := errorMatchRegex.FindStringSubmatch(hint)
	if len(errorMatches) > 1 {
		headline = errorMatches[1]
	}
	return &RebaseResult{
		Status:        status,
		Hint:          normalizeRebaseHint(out.Stderr),
		ErrorHeadline: headline,
	}, nil
}
