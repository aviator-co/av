package git

import (
	"context"
	"strconv"
	"strings"

	"emperror.dev/errors"
)

// Version returns the installed git version as (major, minor). Patch level and
// any build suffix (e.g. "(Apple Git-152)") are ignored.
func (r *Repo) Version(ctx context.Context) (major, minor int, err error) {
	out, err := r.Git(ctx, "--version")
	if err != nil {
		return 0, 0, err
	}
	return parseGitVersion(out)
}

func parseGitVersion(out string) (int, int, error) {
	// e.g. "git version 2.54.0", "git version 2.45.2 (Apple Git-152)"
	fields := strings.Fields(out)
	if len(fields) < 3 || fields[0] != "git" || fields[1] != "version" {
		return 0, 0, errors.Errorf("unexpected git version output: %q", out)
	}
	parts := strings.SplitN(fields[2], ".", 3)
	if len(parts) < 2 {
		return 0, 0, errors.Errorf("unexpected git version string: %q", fields[2])
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, errors.Wrapf(err, "parse git major version %q", parts[0])
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, errors.Wrapf(err, "parse git minor version %q", parts[1])
	}
	return major, minor, nil
}

// HookRunArgs returns the arguments to pass to `git` to run a custom
// (non-native) hook. The --allow-unknown-hook-name flag is appended on git
// versions that support it (>= 2.44); it is accepted on 2.44–2.53 and
// required on 2.54+, which rejects non-native hook names otherwise.
func (r *Repo) HookRunArgs(ctx context.Context, hookName string) []string {
	args := []string{"hook", "run", "--ignore-missing"}
	if r.supportsAllowUnknownHookName(ctx) {
		args = append(args, "--allow-unknown-hook-name")
	}
	return append(args, hookName)
}

func (r *Repo) supportsAllowUnknownHookName(ctx context.Context) bool {
	major, minor, err := r.Version(ctx)
	if err != nil {
		// If we can't determine the version, assume a modern git so that
		// the flag is included and `git hook run` doesn't fail on 2.54+.
		r.log.WithError(err).Debug("failed to determine git version")
		return true
	}
	return major > 2 || (major == 2 && minor >= 44)
}
