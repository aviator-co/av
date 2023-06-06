package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/config"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/reorder"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

var stackReorderFlags struct {
	Continue bool
	Abort    bool
}

const stackReorderDoc = `
Interactively reorder the stack.

This is analogous to git rebase --interactive but operates across all branches
in the stack.

Branches can be re-arranged within the stack and commits can be edited,
squashed, dropped, or moved within the stack.
`

var stackReorderCmd = &cobra.Command{
	Use:    "reorder",
	Short:  "reorder the stack",
	Hidden: true,
	Long:   strings.TrimSpace(stackReorderDoc),
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.Version != config.VersionDev {
			logrus.Fatal("av stack reorder is not yet implemented")
		}
		repo, err := getRepo()
		if err != nil {
			return err
		}
		db, err := getDB(repo)
		if err != nil {
			return err
		}

		continuation, err := reorder.ReadContinuation(repo)
		if os.IsNotExist(err) {
			if stackReorderFlags.Continue || stackReorderFlags.Abort {
				_, _ = fmt.Fprint(os.Stderr,
					colors.Failure("ERROR: no reorder in progress\n"),
				)
				return actions.ErrExitSilently{ExitCode: 127}
			}
		} else if err != nil {
			return err
		}

		var state *reorder.State
		if stackReorderFlags.Abort {
			// TODO: Handle clearing any cherry-pick state and whatnot.
			// TODO: --abort should probably reset the state of each branch
			//   associated with the reorder to the original. It might be worth
			//   storing some history and allow the user to do --undo to restore
			//   their Git state to the state before the reorder.
			return reorder.WriteContinuation(repo, nil)
		} else if stackReorderFlags.Continue {
			state = continuation.State
		} else {
			if continuation != nil {
				_, _ = fmt.Fprint(os.Stderr,
					colors.Failure("ERROR: reorder already in progress\n"),
					colors.Failure("	   use --continue or --abort to continue or abort the reorder\n"),
				)
				return actions.ErrExitSilently{ExitCode: 127}
			}
			tx := db.ReadTx()
			currentBranch, err := repo.CurrentBranchName()
			if err != nil {
				return err
			}
			root, ok := meta.Root(tx, currentBranch)
			if !ok {
				_, _ = fmt.Fprint(os.Stderr,
					colors.Failure("ERROR: branch "), colors.UserInput(currentBranch),
					colors.Failure(" is not part of a stack\n"),
				)
				return actions.ErrExitSilently{ExitCode: 127}
			}
			plan, err := reorder.CreatePlan(repo, db.ReadTx(), root)
			if err != nil {
				return err
			}

			// TODO:
			// What should we do if the plan removes branches? Currently,
			// we just don't edit those branches. So if a user edits
			//     sb one
			//     pick 1a
			//     sb two
			//     pick 2a
			// and deletes the line for `sb two`, then the reorder will modify
			// one to contain 1a/2a, but we don't modify two so it'll still be
			// considered stacked on top of one.
			// We can probably delete the branch and also emit a message to the
			// user to the effect of
			//     Deleting branch `two` because it was removed from the reorder.
			//     To restore the branch, run `git switch -C two <OLD HEAD>`.
			// just to make sure they can recover their work. (They already would
			// be able to using `git reflog` but generally only advanced Git
			// users think to do that).
			plan, err = reorder.EditPlan(repo, plan)
			if err != nil {
				return err
			}
			if len(plan) == 0 {
				_, _ = fmt.Fprint(os.Stderr,
					colors.Failure("ERROR: reorder plan is empty\n"),
				)
				return actions.ErrExitSilently{ExitCode: 127}
			}

			logrus.WithFields(logrus.Fields{
				"plan":           plan,
				"current_branch": currentBranch,
				"root_branch":    root,
			}).Debug("created reorder plan")
			state = &reorder.State{Commands: plan}
		}

		continuation, err = reorder.Reorder(reorder.Context{
			Repo:   repo,
			DB:     db,
			State:  state,
			Output: os.Stderr,
		})
		if err != nil {
			return err
		}
		if continuation == nil {
			_, _ = fmt.Fprint(os.Stderr,
				colors.Success("\nThe stack was reordered successfully.\n"),
			)
			return nil
		}

		if err := reorder.WriteContinuation(repo, continuation); err != nil {
			return err
		}
		_, _ = fmt.Fprint(os.Stderr,
			colors.Warning("\nThe reorder was interrupted by a conflict.\n"),
			colors.Warning("Resolve the conflict and run "),
			colors.CliCmd("av stack reorder --continue"),
			colors.Warning(" to continue.\n"),
		)
		return actions.ErrExitSilently{ExitCode: 1}
	},
}

func init() {
	stackReorderCmd.Flags().
		BoolVar(&stackReorderFlags.Continue, "continue", false, "continue a previous reorder")
	stackReorderCmd.Flags().
		BoolVar(&stackReorderFlags.Abort, "abort", false, "abort a previous reorder")
	stackReorderCmd.MarkFlagsMutuallyExclusive("continue", "abort")
}
