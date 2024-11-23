package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"

	"github.com/aviator-co/av/internal/git"
	"github.com/spf13/cobra"
)

var stackDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show the diff between working tree and parent branch",
	Long: strings.TrimSpace(`
Generates the diff between the working tree and the parent branch 
(i.e., the diff between the current branch and the previous branch in the stack).
`),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}
		db, err := getDB(repo)
		if err != nil {
			return err
		}

		currentBranchName, err := repo.CurrentBranchName()
		if err != nil {
			return err
		}

		tx := db.ReadTx()
		branch, exists := tx.Branch(currentBranchName)
		if !exists {
			defaultBranch, err := repo.DefaultBranch()
			if err != nil {
				return err
			}
			branch.Parent = meta.BranchState{
				Name:  defaultBranch,
				Trunk: true,
			}
		}

		diffArgs := []string{"diff"}
		notUpToDate := false

		if branch.Parent.Trunk {
			// Compare against the merge-base so that we effectively only see the
			// diff associated with this branch. Without this, if main has
			// advanced since this branch was created, we'd also see the (inverse)
			// diff between main and this branch. For example, if we have:
			//     main: X -> Y
			//     one:   \-> 1a -> 1b
			// without --merge-base, we compute the diff between Y and 1b, which
			// shows that we're undoing the changes that were introduced in Y
			// (since 1b doesn't have those changes). With --merge-base, we
			// compute the diff relative to X, which is probably what the user
			// expects.
			// This roughly matches the diff that GitHub will show in the pull
			// request files changed view.
			diffArgs = append(diffArgs, "--merge-base", branch.Parent.Name)
		} else {
			// For a non-root branch, we compare against the original branch point.
			// We don't want to compare exactly against the parent branch since
			// the parent branch might have been modified but not yet synced.
			// For example, if we have:
			//     main: X -> Y
			//     one:   \-> 1a
			//     two:         \-> 2a
			// and then one is updated (either by adding a new commit or amending),
			// we still only want to show the diff associated with two as {2a}.
			// Concretely, if we have
			//     main: X -> Y
			//     one:   \-> 1a -> 1b
			//     two:         \-> 2a
			// we don't want to compute the diff between 1b and 2a since that
			// will show the diff between 1b and 2a, which shows that we're
			// undoing the changes that were introduced in 1b (since 2a doesn't
			// have those changes). Instead, we want to compute the diff between
			// 1a and 2a. We can't just use merge-base here to account for the
			// fact that one might be amended (not just advanced).
			diffArgs = append(diffArgs, branch.Parent.Head)

			// Determine if the branch is up-to-date with the parent and warn if
			// not.
			currentParentHead, err := repo.RevParse(&git.RevParse{Rev: branch.Parent.Name})
			if err != nil {
				return err
			}
			notUpToDate = currentParentHead != branch.Parent.Head
		}

		// NOTE:
		// We don't use repo.Diff here since that sets the --exit-error flag
		// which in turn disables the output pager. We want this command to
		// behave similarly to default `git diff` for the user.
		_, err = repo.Run(&git.RunOpts{
			Args:        diffArgs,
			Interactive: true,
		})
		if err != nil {
			return err
		}

		// We need to display this **after** the diff to ensure that the diff
		// output pager doesn't eat this message.
		if notUpToDate {
			_, _ = fmt.Fprint(os.Stderr,
				colors.Warning("\nWARNING: Branch "), colors.UserInput(currentBranchName),
				colors.Warning(" is not up to date with parent branch "),
				colors.UserInput(branch.Parent.Name), colors.Warning(". Run "),
				colors.CliCmd("av sync"), colors.Warning(" to synchronize the branch.\n"),
			)
			return actions.ErrExitSilently{ExitCode: 1}
		}

		return nil
	},
}
