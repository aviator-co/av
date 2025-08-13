package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/meta/jsonfiledb"
	"github.com/aviator-co/av/internal/meta/refmeta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/spf13/cobra"
)

var cachedRepo *git.Repo

func getRepo(ctx context.Context) (*git.Repo, error) {
	if cachedRepo == nil {
		cmd := exec.CommandContext(
			ctx,
			"git",
			"rev-parse",
			"--path-format=absolute",
			"--show-toplevel",
			"--git-common-dir",
		)

		if rootFlags.Directory != "" {
			cmd.Dir = rootFlags.Directory
		}
		paths, err := cmd.Output()
		if err != nil {
			return nil, errors.Wrap(
				err,
				"failed to find git directory (are you running inside a Git repo?)",
			)
		}

		dir, gitDir, found := strings.Cut(strings.TrimSpace(string(paths)), "\n")
		if !found {
			return nil, errors.New("Unexpected format, not able to parse toplevel and common dir.")
		}

		cachedRepo, err = git.OpenRepo(dir, gitDir)
		if err != nil {
			return nil, errors.Wrap(err, "failed to open git repo")
		}
	}
	return cachedRepo, nil
}

var ErrRepoNotInitialized = errors.Sentinel(
	"this repository is not initialized; please run `av init`",
)

func getDB(ctx context.Context, repo *git.Repo) (meta.DB, error) {
	db, exists, err := getOrCreateDB(ctx, repo)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrRepoNotInitialized
	}
	return db, nil
}

func getOrCreateDB(ctx context.Context, repo *git.Repo) (meta.DB, bool, error) {
	dbPath := filepath.Join(repo.AvDir(), "av.db")
	oldDBPathPath := filepath.Join(repo.AvDir(), "repo-metadata.json")
	dbPathStat, _ := os.Stat(dbPath)
	oldDBPathStat, _ := os.Stat(oldDBPathPath)

	if dbPathStat == nil && oldDBPathStat != nil {
		// Migrate old db to new db
		db, exists, err := jsonfiledb.OpenPath(dbPath)
		if err != nil {
			return nil, false, err
		}
		if err := refmeta.Import(ctx, repo, db); err != nil {
			return nil, false, errors.WrapIff(err, "failed to import ref metadata into av database")
		}
		return db, exists, nil
	}
	return jsonfiledb.OpenPath(dbPath)
}

func allBranches(ctx context.Context) ([]string, error) {
	repo, err := getRepo(ctx)
	if err != nil {
		return nil, err
	}
	db, err := getDB(ctx, repo)
	if err != nil {
		return nil, err
	}

	defaultBranch, err := repo.DefaultBranch(ctx)
	if err != nil {
		return nil, err
	}

	tx := db.ReadTx()

	branches := []string{defaultBranch}
	for b := range tx.AllBranches() {
		branches = append(branches, b)
	}

	return branches, nil
}

// stripRemoteRefPrefixes removes the "refs/heads/", "refs/remotes/<remote>/", "<remote>/" prefix
// from a ref name if it exists.
func stripRemoteRefPrefixes(repo *git.Repo, possibleRefName string) string {
	ret := possibleRefName
	if strings.HasPrefix(ret, "refs/heads/") {
		ret = strings.TrimPrefix(ret, "refs/heads/")
	} else {
		ret = strings.TrimPrefix(ret, "refs/remotes/")
		ret = strings.TrimPrefix(ret, repo.GetRemoteName()+"/")
	}
	return ret
}

// deprecateCommand will create a new version of the command that injects a deprecation message
// into the command's short and long descriptions. As well as a pre-run hook that will print
// a deprecation warning before running the command.
func deprecateCommand(
	cmd cobra.Command,
	newCmd string,
	use string,
) *cobra.Command {
	deprecatedCommand := cmd

	deprecatedCommand.Use = use
	deprecatedCommand.Short = fmt.Sprintf("Deprecated: %s (use '%s' instead)", cmd.Short, newCmd)

	if deprecatedCommand.Long != "" {
		long := fmt.Sprintf("This command is deprecated. Please use '%s' instead.\n\n", newCmd)

		deprecatedCommand.Long = long + deprecatedCommand.Long
	}

	deprecatedCommand.PreRun = func(cmd *cobra.Command, args []string) {
		fmt.Print(
			colors.Warning("This command is deprecated. Please use "),
			colors.CliCmd("'", newCmd, "'"),
			colors.Warning(" instead.\n"),
		)
	}

	return &deprecatedCommand
}

func branchNameArgs(
	cmd *cobra.Command,
	_ []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	branches, _ := allBranches(cmd.Context())
	return branches, cobra.ShellCompDirectiveNoSpace
}

// validateBranchSync checks if the current branch is in sync with its parent branch.
// Only enforces this check for non-trunk branches when we have a stored parent head.
// This validation prevents commands from operating on branches that are out of sync
// with their parent, which could lead to unexpected behavior or loss of commits.
func validateBranchSync(ctx context.Context, repo *git.Repo, branch meta.Branch) error {
	if branch.Parent.Trunk {
		// If there's no parent or the branch is a trunk branch, we don't need to check sync
		return nil
	}

	// Get the current head of the parent branch
	currentParentHead, err := repo.RevParse(ctx, &git.RevParse{Rev: branch.Parent.Name})
	if err != nil {
		return errors.Errorf(
			"cannot get current head of parent branch %s: %v",
			branch.Parent.Name,
			err,
		)
	}

	// If the current parent head differs from the stored parent head,
	// then the parent has moved and we need to sync before proceeding
	if currentParentHead != branch.Parent.Head {
		return errors.Errorf(
			"branch is not in sync with parent branch %s, please run 'av sync' first",
			branch.Parent.Name,
		)
	}

	// Additional check: ensure the child branch is actually based on the stored parent head
	mergeBase, err := repo.MergeBase(ctx, branch.Parent.Head, branch.Name)
	if err != nil {
		return errors.Errorf("cannot find merge base with parent branch: %v", err)
	}

	if mergeBase != branch.Parent.Head {
		return errors.Errorf(
			"branch is not in sync with parent branch %s, please run 'av sync' first",
			branch.Parent.Name,
		)
	}
	return nil
}
