package main

import (
	"os"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/sequencer"
	"github.com/aviator-co/av/internal/sequencer/planner"
	"github.com/aviator-co/av/internal/sequencer/sequencerui"
	"github.com/aviator-co/av/internal/utils/uiutils"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
)

var restackFlags struct {
	All      bool
	Current  bool
	Abort    bool
	Continue bool
	Skip     bool
	DryRun   bool
}

var restackCmd = &cobra.Command{
	Use:   "restack",
	Short: "Rebase the stacked branches",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}
		db, err := getDB(repo)
		if err != nil {
			return err
		}
		return uiutils.RunBubbleTea(&restackViewModel{repo: repo, db: db})
	},
}

type restackViewModel struct {
	repo *git.Repo
	db   meta.DB

	restackModel *sequencerui.RestackModel

	quitWithConflict bool
	err              error
}

func (vm *restackViewModel) Init() tea.Cmd {
	state, err := vm.readState()
	if err != nil {
		return func() tea.Msg { return err }
	}
	if state == nil {
		if restackFlags.Abort || restackFlags.Continue || restackFlags.Skip {
			return func() tea.Msg { return errors.New("no restack in progress") }
		}
		state, err = vm.createState()
		if err != nil {
			return func() tea.Msg { return err }
		}
	}
	if state == nil {
		return func() tea.Msg { return nothingToRestackError }
	}
	vm.restackModel = sequencerui.NewRestackModel(vm.repo, vm.db)
	vm.restackModel.State = state
	vm.restackModel.Abort = restackFlags.Abort
	vm.restackModel.Continue = restackFlags.Continue
	vm.restackModel.Skip = restackFlags.Skip
	vm.restackModel.DryRun = restackFlags.DryRun
	return vm.restackModel.Init()
}

func (vm *restackViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case *sequencerui.RestackProgress, spinner.TickMsg:
		var cmd tea.Cmd
		vm.restackModel, cmd = vm.restackModel.Update(msg)
		return vm, cmd
	case *sequencerui.RestackConflict:
		if err := vm.writeState(vm.restackModel.State); err != nil {
			return vm, func() tea.Msg { return err }
		}
		vm.quitWithConflict = true
		return vm, tea.Quit
	case *sequencerui.RestackAbort, *sequencerui.RestackDone:
		if err := vm.writeState(nil); err != nil {
			return vm, func() tea.Msg { return err }
		}
		return vm, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return vm, tea.Quit
		}
	case error:
		vm.err = msg
		return vm, tea.Quit
	}
	return vm, nil
}

func (vm *restackViewModel) View() string {
	var ss []string
	if vm.restackModel != nil {
		ss = append(ss, vm.restackModel.View())
	}

	var ret string
	if len(ss) != 0 {
		ret = lipgloss.NewStyle().MarginTop(1).MarginBottom(1).MarginLeft(2).Render(
			lipgloss.JoinVertical(0, ss...),
		)
	}
	if vm.err != nil {
		if len(ret) != 0 {
			ret += "\n"
		}
		ret += renderError(vm.err)
	}
	return ret
}

func (vm *restackViewModel) ExitError() error {
	if errors.Is(vm.err, nothingToRestackError) {
		return nil
	}
	if vm.err != nil {
		return actions.ErrExitSilently{ExitCode: 1}
	}
	if vm.quitWithConflict {
		return actions.ErrExitSilently{ExitCode: 1}
	}
	return nil
}

func (vm *restackViewModel) readState() (*sequencerui.RestackState, error) {
	var state sequencerui.RestackState
	if err := vm.repo.ReadStateFile(git.StateFileKindRestack, &state); err != nil &&
		os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &state, nil
}

func (vm *restackViewModel) writeState(state *sequencerui.RestackState) error {
	if state == nil {
		return vm.repo.WriteStateFile(git.StateFileKindRestack, nil)
	}
	return vm.repo.WriteStateFile(git.StateFileKindRestack, state)
}

func (vm *restackViewModel) createState() (*sequencerui.RestackState, error) {
	var state sequencerui.RestackState

	status, err := vm.repo.Status()
	if err != nil {
		return nil, err
	}
	currentBranch := status.CurrentBranch
	state.InitialBranch = currentBranch

	if restackFlags.All {
		state.RestackingAll = true
	} else {
		if _, exist := vm.db.ReadTx().Branch(currentBranch); !exist {
			return nil, errors.New("current branch is not adopted to av")
		}
		state.RelatedBranches = append(state.RelatedBranches, currentBranch)
	}

	var currentBranchRef plumbing.ReferenceName
	if currentBranch != "" {
		currentBranchRef = plumbing.NewBranchReferenceName(currentBranch)
	}

	ops, err := planner.PlanForRestack(
		vm.db.ReadTx(),
		vm.repo,
		currentBranchRef,
		restackFlags.All,
		restackFlags.Current,
	)
	if err != nil {
		return nil, err
	}
	state.Seq = sequencer.NewSequencer(vm.repo.GetRemoteName(), vm.db, ops)
	return &state, nil
}

func init() {
	restackCmd.Flags().BoolVar(
		&restackFlags.All, "all", false,
		"rebase all branches",
	)
	restackCmd.Flags().BoolVar(
		&restackFlags.Current, "current", false,
		"only rebase up to the current branch\n(don't recurse into descendant branches)",
	)
	restackCmd.Flags().BoolVar(
		&restackFlags.Continue, "continue", false,
		"continue an in-progress rebase",
	)
	restackCmd.Flags().BoolVar(
		&restackFlags.Abort, "abort", false,
		"abort an in-progress rebase",
	)
	restackCmd.Flags().BoolVar(
		&restackFlags.Skip, "skip", false,
		"skip the current commit and continue an in-progress rebase",
	)
	restackCmd.Flags().BoolVar(
		&restackFlags.DryRun, "dry-run", false,
		"show the list of branches that will be rebased without actually rebasing them",
	)

	restackCmd.MarkFlagsMutuallyExclusive("continue", "abort", "skip")
}
