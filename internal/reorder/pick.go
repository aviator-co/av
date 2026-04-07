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
		if err := p.PerformSquash(context.Background(), ctx.Repo); err != nil {
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
func (p PickCmd) PerformSquash(ctx context.Context, repo *git.Repo) error {
	var amendArgs []string
	if p.Mode == PickModeFixup {
		amendArgs = []string{"commit", "--amend", "--no-edit"}
	} else {
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
			return errors.New("squash commit message is empty after editing")
		}
		amendArgs = []string{"commit", "--amend", "--message", editedMsg}
	}

	// Undo the cherry-picked commit while keeping its changes staged, then
	// amend the previous commit to incorporate those changes.
	if _, err := repo.Run(ctx, &git.RunOpts{
		Args:      []string{"reset", "--soft", "HEAD~1"},
		ExitError: true,
	}); err != nil {
		return err
	}

	if _, err := repo.Run(ctx, &git.RunOpts{
		Args:      amendArgs,
		ExitError: true,
	}); err != nil {
		return err
	}

	return nil
}

func getCommitMessage(ctx context.Context, repo *git.Repo, rev string) (string, error) {
	out, err := repo.Run(ctx, &git.RunOpts{
		Args:      []string{"log", "-1", "--format=%B", rev},
		ExitError: true,
	})
	if err != nil {
		return "", err
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
	sb.WriteString(p.Commit)
	if p.Comment != "" {
		sb.WriteString("  # ")
		sb.WriteString(p.Comment)
	}
	return sb.String()
}

var _ Cmd = &PickCmd{}

func parsePickCmd(args []string) (Cmd, error) {
	if len(args) != 1 {
		return nil, ErrInvalidCmd{"pick", "exactly one argument is required (the commit to pick)"}
	}
	return PickCmd{Commit: args[0]}, nil
}

func parseSquashCmd(args []string) (Cmd, error) {
	if len(args) != 1 {
		return nil, ErrInvalidCmd{"squash", "exactly one argument is required (the commit to squash)"}
	}
	return PickCmd{Commit: args[0], Mode: PickModeSquash}, nil
}

func parseFixupCmd(args []string) (Cmd, error) {
	if len(args) != 1 {
		return nil, ErrInvalidCmd{"fixup", "exactly one argument is required (the commit to fixup)"}
	}
	return PickCmd{Commit: args[0], Mode: PickModeFixup}, nil
}
