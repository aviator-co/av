package main

import (
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

func getRepo() (*git.Repo, error) {
	if cachedRepo == nil {
		cmd := exec.Command(
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

func getDB(repo *git.Repo) (meta.DB, error) {
	db, exists, err := getOrCreateDB(repo)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrRepoNotInitialized
	}
	return db, nil
}

func getOrCreateDB(repo *git.Repo) (meta.DB, bool, error) {
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
		if err := refmeta.Import(repo, db); err != nil {
			return nil, false, errors.WrapIff(err, "failed to import ref metadata into av database")
		}
		return db, exists, nil
	}
	return jsonfiledb.OpenPath(dbPath)
}

func allBranches() ([]string, error) {
	repo, err := getRepo()
	if err != nil {
		return nil, err
	}
	db, err := getDB(repo)
	if err != nil {
		return nil, err
	}

	tx := db.ReadTx()

	var branches []string
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
		var long = fmt.Sprintf("This command is deprecated. Please use '%s' instead.\n\n", newCmd)

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
	_ *cobra.Command,
	_ []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	branches, _ := allBranches()
	return branches, cobra.ShellCompDirectiveNoSpace
}
