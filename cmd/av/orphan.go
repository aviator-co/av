package main

import (
	"fmt"
	"os"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/uiutils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var orphanFlags struct {
	Yes bool
}

var orphanCmd = &cobra.Command{
	Use:   "orphan",
	Short: "Orphan branches that are managed by av",
	Args:  cobra.NoArgs,
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

		branchesToOrphan, err := collectBranchesToOrphan(repo, db.ReadTx())
		if err != nil {
			return err
		}

		if len(branchesToOrphan) > 1 && !orphanFlags.Yes {
			return uiutils.RunBubbleTea(&orphanConfirmViewModel{
				db:               db,
				branchesToOrphan: branchesToOrphan,
			})
		}

		return orphanBranches(db, branchesToOrphan)
	},
}

type orphanConfirmViewModel struct {
	db               meta.DB
	branchesToOrphan []string

	uiutils.BaseStackedView
}

type orphanedBranchesMsg struct{}

func (vm *orphanConfirmViewModel) Init() tea.Cmd {
	return vm.AddView(&uiutils.NewlineModel{Model: uiutils.NewPromptModel(
		orphanConfirmPrompt(vm.branchesToOrphan),
		[]string{"Yes", "No"},
		func(choice string) tea.Cmd {
			if choice == "No" {
				return tea.Quit
			}
			return vm.orphanBranches()
		},
	)})
}

func (vm *orphanConfirmViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case orphanedBranchesMsg:
		return vm, tea.Quit
	}
	return vm, vm.BaseStackedView.Update(msg)
}

func (vm *orphanConfirmViewModel) View() string {
	return vm.BaseStackedView.View()
}

func (vm *orphanConfirmViewModel) ExitError() error {
	if vm.Err != nil {
		return actions.ErrExitSilently{ExitCode: 1}
	}
	return nil
}

func (vm *orphanConfirmViewModel) orphanBranches() tea.Cmd {
	return func() tea.Msg {
		if err := orphanBranches(vm.db, vm.branchesToOrphan); err != nil {
			return err
		}
		return orphanedBranchesMsg{}
	}
}

func orphanConfirmPrompt(branchesToOrphan []string) string {
	return fmt.Sprintf(
		"Orphaning %d branches will stop av from tracking: %s. Re-adoption is one-by-one. Continue?",
		len(branchesToOrphan),
		strings.Join(branchesToOrphan, ", "),
	)
}

func collectBranchesToOrphan(repo *git.Repo, tx meta.ReadTx) ([]string, error) {
	currentBranch, err := repo.CurrentBranchName()
	if err != nil {
		return nil, errors.WrapIf(err, "failed to determine current branch")
	}

	branchesToOrphan := []string{currentBranch}
	branchesToOrphan = append(branchesToOrphan, meta.SubsequentBranches(tx, currentBranch)...)
	return branchesToOrphan, nil
}

func orphanBranches(db meta.DB, branchesToOrphan []string) error {
	tx := db.WriteTx()
	defer tx.Abort()

	for _, branch := range branchesToOrphan {
		tx.DeleteBranch(branch)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	fmt.Fprintf(
		os.Stderr,
		"These branched are orphaned: %s\n",
		strings.Join(branchesToOrphan, ", "),
	)

	return nil
}

func init() {
	orphanCmd.Flags().BoolVarP(
		&orphanFlags.Yes,
		"yes",
		"y",
		false,
		"orphan multiple branches without prompting for confirmation",
	)
}
