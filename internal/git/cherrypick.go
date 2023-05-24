package git

import (
	"emperror.dev/errors"
	"fmt"
	"strings"
)

type CherryPickResume string

const (
	CherryPickContinue CherryPickResume = "continue"
	CherryPickSkip     CherryPickResume = "skip"
	CherryPickQuit     CherryPickResume = "quit"
	CherryPickAbort    CherryPickResume = "abort"
)

type CherryPick struct {
	// Commits is a list of commits to apply.
	Commits []string

	// NoCommit specifies whether or not to cherry-pick without committing
	// (equivalent to the --no-commit flag on `git cherry-pick`).
	NoCommit bool

	// FastForward specifies whether or not to fast-forward the current branch
	// if possible (equivalent to the --ff flag on `git cherry-pick`).
	// If true, and the parent of the commit is the current HEAD, the HEAD
	// will be fast forwarded to the commit (instead of re-applied).
	FastForward bool

	// Resume specifies how to resume a cherry-pick operation that was
	// interrupted by a conflict (equivalent to the --continue, --skip, --quit,
	// and --abort flags on `git cherry-pick`).
	// Mutually exclusive with all other options.
	Resume CherryPickResume
}

type CherryPickResult struct {
	// Head is the last commit that was successfully cherry-picked.
	Head string

	// Conflict is true if the cherry-pick operation resulted in a conflict.
	Conflict bool

	// CherryPickHead is the commit that was unable to be applied.
	// Set only if Conflict is true.
	CherryPickHead string
}

type ErrCherryPickConflict struct {
	ConflictingCommit string
	Output            string
}

func (e ErrCherryPickConflict) Error() string {
	return fmt.Sprintf("cherry-pick conflict: failed to apply %s", ShortSha(e.ConflictingCommit))
}

// CherryPick applies the given commits on top of the current HEAD.
// If there are conflicts, ErrCherryPickConflict is returned.
func (r *Repo) CherryPick(opts CherryPick) error {
	args := []string{"cherry-pick"}

	if opts.Resume != "" {
		args = append(args, fmt.Sprintf("--%s", opts.Resume))
	} else {
		if opts.FastForward {
			args = append(args, "--ff")
		}
		if opts.NoCommit {
			args = append(args, "--no-commit")
		}
		args = append(args, opts.Commits...)
	}

	run, err := r.Run(&RunOpts{
		Args: args,
	})
	if err != nil {
		return err
	}

	if run.ExitCode != 0 {
		cherryPickHead, err := r.readGitFile("CHERRY_PICK_HEAD")
		if err != nil {
			return errors.WrapIff(err, "expected CHERRY_PICK_HEAD to exist after cherry-pick failure")
		}
		return ErrCherryPickConflict{
			ConflictingCommit: strings.TrimSpace(cherryPickHead),
			Output:            string(run.Stderr),
		}
	}

	return nil
}
