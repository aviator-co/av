package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/reorder"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var reorderFlags struct {
	Continue bool
	Abort    bool
}

var reorderCmd = &cobra.Command{
	Use:   "reorder",
	Short: "Interactively reorder the stack",
	Long: strings.TrimSpace(`
Interactively reorder the stack.

This is analogous to git rebase --interactive but operates across all branches
in the stack.

Branches can be re-arranged within the stack and commits can be edited,
squashed, dropped, or moved within the stack.
`),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		repo, err := getRepo(ctx)
		if err != nil {
			return err
		}
		db, err := getDB(ctx, repo)
		if err != nil {
			return err
		}

		var continuation reorder.Continuation
		if err := repo.ReadStateFile(git.StateFileKindReorder, &continuation); os.IsNotExist(err) {
			if reorderFlags.Continue || reorderFlags.Abort {
				fmt.Fprint(os.Stderr,
					colors.Failure("ERROR: no reorder in progress\n"),
				)
				return actions.ErrExitSilently{ExitCode: 127}
			}
		} else if err != nil {
			return err
		}

		var state *reorder.State
		if reorderFlags.Abort {
			if continuation.State == nil {
				_ = repo.WriteStateFile(git.StateFileKindReorder, nil)
				return errors.New("no reorder in progress")
			}

			if stat, _ := os.Stat(filepath.Join(repo.GitDir(), "CHERRY_PICK_HEAD")); stat != nil {
				if err := repo.CherryPick(ctx, git.CherryPick{Resume: git.CherryPickAbort}); err != nil {
					return errors.WrapIf(err, "failed to abort in-progress cherry-pick")
				}
			}
			// TODO: --abort should probably reset the state of each branch
			//   associated with the reorder to the original. It might be worth
			//   storing some history and allow the user to do --undo to restore
			//   their Git state to the state before the reorder.
			return repo.WriteStateFile(git.StateFileKindReorder, nil)
		} else if reorderFlags.Continue {
			state = continuation.State
		} else {
			if continuation.State != nil {
				fmt.Fprint(os.Stderr,
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
				fmt.Fprint(os.Stderr,
					colors.Failure("ERROR: branch "), colors.UserInput(currentBranch),
					colors.Failure(" is not part of a stack\n"),
				)
				return actions.ErrExitSilently{ExitCode: 127}
			}
			initialPlan, err := reorder.CreatePlan(ctx, repo, db.ReadTx(), root)
			if err != nil {
				return err
			}

			plan, err := reorderEditPlan(ctx, repo, initialPlan)
			if err != nil {
				return err
			}

			logrus.WithFields(logrus.Fields{
				"plan":           plan,
				"current_branch": currentBranch,
				"root_branch":    root,
			}).Debug("created reorder plan")
			state = &reorder.State{Commands: plan}
		}

		state, err = reorder.Reorder(reorder.Context{
			Repo:   repo,
			DB:     db,
			State:  state,
			Output: os.Stderr,
		})
		if err != nil {
			return err
		}
		if state == nil {
			if err := repo.WriteStateFile(git.StateFileKindReorder, nil); err != nil {
				return err
			}
			fmt.Fprint(os.Stderr,
				colors.Success("\nThe stack was reordered successfully.\n"),
			)
			return nil
		}

		continuation = reorder.Continuation{State: state}
		if err := repo.WriteStateFile(git.StateFileKindReorder, &continuation); err != nil {
			return err
		}
		fmt.Fprint(os.Stderr,
			colors.Warning("\nThe reorder was interrupted by a conflict.\n"),
			colors.Warning("Resolve the conflict and run "),
			colors.CliCmd("av reorder --continue"),
			colors.Warning(" to continue.\n"),
		)
		return actions.ErrExitSilently{ExitCode: 1}
	},
}

func init() {
	reorderCmd.Flags().
		BoolVar(&reorderFlags.Continue, "continue", false, "continue an in-progress reorder")
	reorderCmd.Flags().
		BoolVar(&reorderFlags.Abort, "abort", false, "abort an in-progress reorder")
	reorderCmd.MarkFlagsMutuallyExclusive("continue", "abort")
}

func reorderEditPlan(
	ctx context.Context,
	repo *git.Repo,
	initialPlan []reorder.Cmd,
) ([]reorder.Cmd, error) {
	plan := initialPlan
edit:
	plan, err := reorder.EditPlan(ctx, repo, plan)
	if err != nil {
		return nil, err
	}
	if len(plan) == 0 {
		fmt.Fprint(os.Stderr,
			colors.Failure("ERROR: reorder plan is empty\n"),
		)
		return nil, actions.ErrExitSilently{ExitCode: 127}
	}

	diff := reorder.Diff(initialPlan, plan)
	if len(diff.RemovedBranches) > 0 {
		fmt.Fprint(
			os.Stderr,
			colors.Warning("\nWARNING: the following branches were removed from the reorder:\n"),
		)
		for _, branch := range diff.RemovedBranches {
			fmt.Fprint(os.Stderr, "  - ", colors.UserInput(branch), "\n")
		}

	promptDeletionBehavior:
		fmt.Fprint(os.Stderr, "\n",
			`What would you like to do?
    [a] Abort the reorder
    [d] Delete the branches
    [e] Edit the reorder plan
    [o] Orphan the branches (the Git branch will continue to exist but will not
        be tracked by av).

[a/d/e/o]: `)
		choice, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return nil, err
		}
		var deleteRefs bool
		// ReadString includes the newline in the string, so this should
		// never panic even if the user just hits enter.
		switch strings.ToLower(string(choice[0])) {
		case "a":
			fmt.Fprint(os.Stderr, colors.Failure("\nAborting reorder.\n"))
			return nil, actions.ErrExitSilently{ExitCode: 127}
		case "d":
			deleteRefs = true
		case "e":
			goto edit
		case "o":
			deleteRefs = false
		default:
			fmt.Fprint(os.Stderr, colors.Failure("\nInvalid choice.\n"))
			goto promptDeletionBehavior
		}

		for _, branch := range diff.RemovedBranches {
			plan = append(plan, reorder.DeleteBranchCmd{Name: branch, DeleteGitRef: deleteRefs})
		}
	}

	return plan, nil
}
