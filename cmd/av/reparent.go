package main

import (
	"context"

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

var reparentFlags struct {
	Parent string
}

var reparentCmd = &cobra.Command{
	Use:               "reparent",
	Short:             "Change the parent of the current branch",
	ValidArgsFunction: branchNameArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		repo, err := getRepo(ctx)
		if err != nil {
			return err
		}

		db, err := getDB(ctx, repo)
		if err != nil {
			return err
		}

		if reparentFlags.Parent != "" && len(args) > 0 && args[0] != "" {
			if reparentFlags.Parent != args[0] {
				return errors.New("conflicting parent branch names")
			}
		}
		if len(args) > 0 && args[0] != "" {
			reparentFlags.Parent = args[0]
		}
		if reparentFlags.Parent == "" {
			return errors.New("missing parent branch name")
		}
		reparentFlags.Parent = stripRemoteRefPrefixes(repo, reparentFlags.Parent)

		return uiutils.RunBubbleTea(&reparentViewModel{repo: repo, db: db})
	},
}

type reparentViewModel struct {
	repo *git.Repo
	db   meta.DB

	restackModel *sequencerui.RestackModel

	quitWithConflict bool
	err              error
}

func (vm *reparentViewModel) Init() tea.Cmd {
	state, err := vm.createState()
	if err != nil {
		return func() tea.Msg { return err }
	}
	vm.restackModel = sequencerui.NewRestackModel(vm.repo, vm.db)
	vm.restackModel.State = state
	return vm.restackModel.Init()
}

func (vm *reparentViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (vm *reparentViewModel) View() string {
	var ss []string
	ss = append(ss, "Reparenting onto "+reparentFlags.Parent+"...")
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

func (vm *reparentViewModel) writeState(state *sequencerui.RestackState) error {
	if state == nil {
		return vm.repo.WriteStateFile(git.StateFileKindRestack, nil)
	}
	return vm.repo.WriteStateFile(git.StateFileKindRestack, state)
}

func (vm *reparentViewModel) createState() (*sequencerui.RestackState, error) {
	ctx := context.Background()
	currentBranch, err := vm.repo.CurrentBranchName(ctx)
	if err != nil {
		return nil, err
	}
	if isCurrentBranchTrunk, err := vm.repo.IsTrunkBranch(ctx, currentBranch); err != nil {
		return nil, err
	} else if isCurrentBranchTrunk {
		return nil, errors.New("current branch is a trunk branch")
	}
	if _, exist := vm.db.ReadTx().Branch(currentBranch); !exist {
		return nil, errors.New("current branch is not adopted to av")
	}

	if isParentBranchTrunk, err := vm.repo.IsTrunkBranch(ctx, reparentFlags.Parent); err != nil {
		return nil, err
	} else if !isParentBranchTrunk {
		if _, exist := vm.db.ReadTx().Branch(reparentFlags.Parent); !exist {
			return nil, errors.New("parent branch is not adopted to av")
		}
	}
	var state sequencerui.RestackState
	state.InitialBranch = currentBranch
	state.RelatedBranches = []string{currentBranch, reparentFlags.Parent}
	ops, err := planner.PlanForReparent(
		ctx,
		vm.db.ReadTx(),
		vm.repo,
		plumbing.NewBranchReferenceName(currentBranch),
		plumbing.NewBranchReferenceName(reparentFlags.Parent),
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

func (vm *reparentViewModel) ExitError() error {
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

func init() {
	reparentCmd.Flags().StringVar(
		&reparentFlags.Parent, "parent", "",
		"parent branch to rebase onto",
	)

	_ = reparentCmd.RegisterFlagCompletionFunc(
		"parent",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			branches, _ := allBranches(cmd.Context())
			return branches, cobra.ShellCompDirectiveNoFileComp
		},
	)
}
