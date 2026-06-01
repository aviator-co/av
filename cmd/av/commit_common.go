package main

import (
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/sequencer"
	"github.com/aviator-co/av/internal/sequencer/planner"
	"github.com/aviator-co/av/internal/sequencer/sequencerui"
	"github.com/aviator-co/av/internal/utils/uiutils"
	"github.com/go-git/go-git/v5/plumbing"
)

var nothingToRestackError = errors.Sentinel("nothing to restack")

func runPostCommitRestack(repo *git.Repo, db meta.DB) error {
	return uiutils.RunBubbleTea(&postCommitRestackViewModel{repo: repo, db: db})
}

type postCommitRestackViewModel struct {
	repo *git.Repo
	db   meta.DB

	state        *sequencerui.RestackState
	restackModel tea.Model

	quitWithConflict bool
	err              error
}

func (vm *postCommitRestackViewModel) Init() tea.Cmd {
	var err error
	vm.state, err = vm.createState()
	if err != nil {
		return uiutils.ErrCmd(err)
	}
	vm.restackModel = sequencerui.NewRestackModel(vm.repo, vm.db, vm.state, sequencerui.RestackStateOptions{
		OnConflict: func() tea.Cmd {
			if err := vm.writeState(vm.state); err != nil {
				return uiutils.ErrCmd(err)
			}
			vm.quitWithConflict = true
			return tea.Quit
		},
		OnAbort: func() tea.Cmd {
			if err := vm.writeState(nil); err != nil {
				return uiutils.ErrCmd(err)
			}
			return tea.Quit
		},
		OnDone: func() tea.Cmd {
			if err := vm.writeState(nil); err != nil {
				return uiutils.ErrCmd(err)
			}
			return tea.Quit
		},
	})
	return vm.restackModel.Init()
}

func (vm *postCommitRestackViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case *sequencerui.RestackProgress, spinner.TickMsg:
		var cmd tea.Cmd
		vm.restackModel, cmd = vm.restackModel.Update(msg)
		return vm, cmd
	case tea.KeyPressMsg:
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

func (vm *postCommitRestackViewModel) View() tea.View {
	sb := strings.Builder{}
	if vm.restackModel != nil {
		sb.WriteString(vm.restackModel.View().Content)
	}
	if vm.err != nil {
		sb.WriteString(vm.err.Error() + "\n")
	}
	return tea.NewView(sb.String())
}

func (vm *postCommitRestackViewModel) ExitError() error {
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

func (vm *postCommitRestackViewModel) writeState(state *sequencerui.RestackState) error {
	if state == nil {
		return vm.repo.WriteStateFile(git.StateFileKindRestack, nil)
	}
	return vm.repo.WriteStateFile(git.StateFileKindRestack, state)
}

func (vm *postCommitRestackViewModel) createState() (*sequencerui.RestackState, error) {
	currentBranch, err := vm.repo.CurrentBranchName()
	if err != nil {
		return nil, err
	}
	if _, exist := vm.db.ReadTx().Branch(currentBranch); !exist {
		return nil, errors.New("current branch is not adopted to av")
	}
	var state sequencerui.RestackState
	state.InitialBranch = currentBranch
	state.RelatedBranches = []string{currentBranch}
	ops, err := planner.PlanForAmend(
		vm.db.ReadTx(),
		vm.repo,
		plumbing.NewBranchReferenceName(currentBranch),
	)
	if err != nil {
		return nil, err
	}
	if len(ops) == 0 {
		return nil, nothingToRestackError
	}
	state.Seq = sequencer.NewSequencer(vm.repo.GetRemoteName(), vm.db, ops)
	return &state, nil
}
