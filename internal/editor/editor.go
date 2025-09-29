package editor

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/kballard/go-shellquote"
	"github.com/sirupsen/logrus"
)

type Config struct {
	// The text to be edited.
	// After the editor is closed, the contents will be written back to this field.
	Text string
	// The file pattern to use when creating the temporary file for the editor.
	TmpFilePattern string
	// The prefix used to identify comments in the text.
	CommentPrefix string
	// If true, strip comments from the end of lines. If false, only whole lines
	// that are comments will be stripped.
	EndOfLineComments bool
	// The editor command to be used.
	// If empty, the git default editor will be used.
	Command string
}

// CommandNoOp is a special command that indicates that no editor should be
// launched and the text should be returned as-is.
// This behavior is copied from git's GIT_EDITOR.
// https://github.com/git/git/blob/5699ec1b0aec51b9e9ba5a2785f65970c5a95d84/editor.c#L57
const CommandNoOp = ":"

// Launch launches the user's editor and allows them to edit the text.
// The text is returned after the editor is closed. If an error occurs, the
// (possibly edited) text is returned in addition to the error. If the file
// could not be read, an empty string is returned.
func Launch(ctx context.Context, repo *git.Repo, config Config) (string, error) {
	switch {
	case config.Command == "":
		config.Command = DefaultCommand(ctx, repo)
	case config.TmpFilePattern == "":
		config.TmpFilePattern = "av-message-*"
	}

	if config.Command == CommandNoOp {
		return config.Text, nil
	}

	tmp, err := os.CreateTemp("", config.TmpFilePattern)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := os.Remove(tmp.Name()); err != nil {
			logrus.WithError(err).Warn("failed to remove temporary file")
		}
	}()
	if _, err := tmp.WriteString(config.Text); err != nil {
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	// Launch the editor as a subprocess.
	// We execute the command through a shell to properly handle environment
	// variable expansion and shell quoting rules, matching Git's behavior.
	// e.g., EDITOR="'/path/with spaces/editor'" or
	// EDITOR="code --wait" or GIT_EDITOR="$EDITOR" work correctly.
	shellCmd := config.Command + " " + shellquote.Join(tmp.Name())
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", shellCmd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	stderr := bytes.NewBuffer(nil)
	cmd.Stderr = stderr
	logrus.WithField("cmd", cmd.String()).Debug("launching editor")
	if err := cmd.Run(); err != nil {
		// Try to return the contents of the file even if the editor exited with
		// an error. We ignore any errors from parsing here since we'll just end
		// up returning the error from above anyway.
		res, _ := parseResult(tmp.Name(), config)
		return res, errors.WrapIff(err, "command %q failed", config.Command)
	}

	return parseResult(tmp.Name(), config)
}

func DefaultCommand(ctx context.Context, repo *git.Repo) string {
	editor, err := repo.Git(ctx, "var", "GIT_EDITOR")
	if err != nil {
		logrus.WithError(err).Warn("failed to determine desired editor from git config")
		// This is the default hard-coded into git
		return "vi"
	}
	return editor
}

func parseResult(path string, config Config) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	scan := bufio.NewScanner(f)
	res := bytes.NewBuffer(nil)
	for scan.Scan() {
		line := scan.Text()
		if strings.HasPrefix(line, config.CommentPrefix) {
			// Skip this line altogether (including the newline).
			continue
		}
		// This currently doesn't include any way to escape comments, but that's
		// probably fine for us for now.
		if config.EndOfLineComments {
			line, _, _ = strings.Cut(line, config.CommentPrefix)
		}
		res.WriteString(line)
		res.WriteByte('\n')
	}
	return res.String(), nil
}
