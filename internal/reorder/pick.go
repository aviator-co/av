package reorder

import (
	"context"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/editor"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/errutils"
	"github.com/kr/text"
)

// ErrEmptySquashMessage is returned by PerformSquash when the user saves an
// empty commit message in the editor during a squash. Callers may detect this
// sentinel to provide a friendlier recovery path (e.g., returning
// ErrInterruptReorder so the user can retry with --continue).
var ErrEmptySquashMessage = errors.New("squash commit message is empty after editing")

// PickMode represents how a commit is applied during reorder.
type PickMode string

const (
	// PickModePick applies the commit as-is (default).
	PickModePick PickMode = ""
	// PickModeSquash squashes the commit into the previous commit,
	// opening the editor to combine the commit messages.
	PickModeSquash PickMode = "squash"
	// PickModeFixup squashes the commit into the previous commit,
	// keeping only the previous commit's message.
	PickModeFixup PickMode = "fixup"
)

// PickCmd is a command that picks a commit from the history and applies it on
// top of the current HEAD.
type PickCmd struct {
	Commit  string
	Comment string
	Mode    PickMode
}

func (p PickCmd) Execute(ctx *Context) error {
	err := ctx.Repo.CherryPick(context.Background(), git.CherryPick{
		Commits: []string{p.Commit},
		// Only use fast-forward for regular picks; squash/fixup need to amend
		// the previous commit after cherry-picking, so fast-forward must be
		// disabled to ensure the cherry-picked commit is always materialized.
		FastForward: p.Mode == PickModePick,
	})
	if conflict, ok := errutils.As[git.ErrCherryPickConflict](err); ok {
		ctx.Print(
			colors.Failure("  - ", conflict.Error(), "\n"),
			colors.Faint(text.Indent(strings.TrimRight(conflict.Output, "\n"), "        "), "\n"),
		)
		return ErrInterruptReorder
	} else if err != nil {
		return err
	}

	if p.Mode != PickModePick {
		if err := p.PerformSquash(context.Background(), ctx.Repo, ctx.State.BranchBase); err != nil {
			if errors.Is(err, ErrEmptySquashMessage) {
				ctx.Print(
					colors.Failure("  - squash commit message is empty after editing\n"),
					colors.Warning("    Edit the message and run "),
					colors.CliCmd("av reorder --continue"),
					colors.Warning(" to retry.\n"),
				)
				return ErrInterruptReorder
			}
			return err
		}
	}

	head, err := ctx.Repo.RevParse(context.Background(), &git.RevParse{Rev: "HEAD"})
	if err != nil {
		return err
	}
	ctx.Print(
		colors.Success("  - applied commit "),
		colors.UserInput(git.ShortSha(p.Commit)),
		colors.Success(" without conflict (HEAD is now at "),
		colors.UserInput(git.ShortSha(head)),
		colors.Success(")\n"),
	)
	ctx.State.Head = head
	return nil
}

// PerformSquash folds the current HEAD commit into HEAD~1.
// For PickModeFixup, the previous commit's message is kept unchanged.
// For PickModeSquash, the editor is opened to compose the combined commit message.
// Must only be called after the commit has been cherry-picked and when Mode != PickModePick.
// branchBase is the commit hash that the current branch was initialized to (from
// State.BranchBase). It is used to prevent folding across the branch boundary
// into a commit that belongs to the parent branch. Pass "" to skip this check
// (e.g., in tests that do not exercise branch-boundary behavior).
func (p PickCmd) PerformSquash(ctx context.Context, repo *git.Repo, branchBase string) error {
	// Guard: HEAD~1 must exist and must not be the branch base commit.
	// If HEAD~1 doesn't exist, this is an orphan commit with no parent to fold
	// into. If HEAD~1 equals branchBase, the cherry-picked commit is the first
	// pick in this branch section and folding would amend the parent branch's
	// tip instead of a commit within this branch.
	parentHash, err := repo.RevParse(ctx, &git.RevParse{Rev: "HEAD~1"})
	if err != nil {
		// rev-parse fails when HEAD is an orphan commit with no parent.
		return errors.New(
			"squash/fixup cannot be applied to the first commit in a branch" +
				" — there is no previous commit to fold into",
		)
	}
	if branchBase != "" && parentHash == branchBase {
		return errors.New(
			"squash/fixup cannot be applied to the first commit in a branch" +
				" — there is no previous commit within this branch to fold into",
		)
	}

	var amendArgs []string
	switch p.Mode {
	case PickModePick:
		return errors.New("PerformSquash called with pick mode — squash/fixup mode is required")
	case PickModeFixup:
		amendArgs = []string{"commit", "--amend", "--no-edit", "--no-verify"}
	case PickModeSquash:
		// Squash: open the editor with both commit messages so the user can
		// compose the combined message.
		prevMsg, err := getCommitMessage(ctx, repo, "HEAD~1")
		if err != nil {
			return err
		}
		squashMsg, err := getCommitMessage(ctx, repo, "HEAD")
		if err != nil {
			return err
		}
		template := "# This is a combination of 2 commits.\n" +
			"# The first commit's message is:\n\n" +
			prevMsg + "\n\n" +
			"# This is the 2nd commit message:\n\n" +
			squashMsg + "\n"
		editedMsg, err := editor.Launch(ctx, repo, editor.Config{
			Text:          template,
			CommentPrefix: "#",
		})
		if err != nil {
			return err
		}
		editedMsg = strings.TrimSpace(editedMsg)
		if editedMsg == "" {
			return ErrEmptySquashMessage
		}
		amendArgs = []string{"commit", "--amend", "--no-verify", "--message", editedMsg}
	default:
		return errors.Errorf("PerformSquash called with unexpected mode %q", p.Mode)
	}

	// Undo the cherry-picked commit while keeping its changes staged, then
	// amend the previous commit to incorporate those changes.
	if _, err := repo.Run(ctx, &git.RunOpts{
		Args:      []string{"reset", "--soft", "HEAD~1"},
		ExitError: true,
	}); err != nil {
		return errors.WrapIff(err, "resetting HEAD before %s of commit %s", p.Mode, p.Commit)
	}

	if _, err := repo.Run(ctx, &git.RunOpts{
		Args:      amendArgs,
		ExitError: true,
	}); err != nil {
		return errors.WrapIff(err, "amending commit during %s of %s", p.Mode, p.Commit)
	}

	return nil
}

func getCommitMessage(ctx context.Context, repo *git.Repo, rev string) (string, error) {
	out, err := repo.Run(ctx, &git.RunOpts{
		Args:      []string{"log", "-1", "--format=%B", rev},
		ExitError: true,
	})
	if err != nil {
		return "", errors.WrapIff(err, "getting commit message for %s", rev)
	}
	return strings.TrimRight(string(out.Stdout), "\n"), nil
}

func (p PickCmd) String() string {
	sb := strings.Builder{}
	mode := string(p.Mode)
	if mode == "" {
		mode = "pick"
	}
	sb.WriteString(mode)
	sb.WriteString(" ")
	sb.WriteString(git.ShortSha(p.Commit))
	if p.Comment != "" {
		sb.WriteString("  # ")
		sb.WriteString(p.Comment)
	}
	return sb.String()
}

var _ Cmd = &PickCmd{}

func parsePickCmd(args []string, shortToFull map[string]string) (Cmd, error) {
	if len(args) != 1 {
		return nil, ErrInvalidCmd{"pick", "exactly one argument is required (the commit to pick)"}
	}
	return PickCmd{Commit: resolveHash(args[0], shortToFull)}, nil
}

func parseSquashCmd(args []string, shortToFull map[string]string) (Cmd, error) {
	if len(args) != 1 {
		return nil, ErrInvalidCmd{"squash", "exactly one argument is required (the commit to squash)"}
	}
	return PickCmd{Commit: resolveHash(args[0], shortToFull), Mode: PickModeSquash}, nil
}

func parseFixupCmd(args []string, shortToFull map[string]string) (Cmd, error) {
	if len(args) != 1 {
		return nil, ErrInvalidCmd{"fixup", "exactly one argument is required (the commit to fixup)"}
	}
	return PickCmd{Commit: resolveHash(args[0], shortToFull), Mode: PickModeFixup}, nil
}
