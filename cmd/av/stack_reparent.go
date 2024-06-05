package main

import (
	"os"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/sequencer"
	"github.com/aviator-co/av/internal/sequencer/planner"
	"github.com/aviator-co/av/internal/sequencer/sequencerui"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var stackReparentFlags struct {
	Parent   string
	Abort    bool
	Continue bool
	Skip     bool
}

var stackReparentCmd = &cobra.Command{
	Use:   "reparent",
	Short: "Reparent branches",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := getRepo()
		if err != nil {
			return err
		}

		db, err := getDB(repo)
		if err != nil {
			return err
		}

		var opts []tea.ProgramOption
		if !isatty.IsTerminal(os.Stdout.Fd()) {
			opts = []tea.ProgramOption{
				tea.WithInput(nil),
			}
		}
		p := tea.NewProgram(&stackReparentViewModel{repo: repo, db: db}, opts...)
		model, err := p.Run()
		if err != nil {
			return err
		}
		if err := model.(*stackReparentViewModel).err; err != nil {
			if errors.Is(err, nothingToRestackError) {
				return nil
			}
			return actions.ErrExitSilently{ExitCode: 1}
		}
		if model.(*stackReparentViewModel).quitWithConflict {
			return actions.ErrExitSilently{ExitCode: 1}
		}
		return nil
	},
}

type stackReparentViewModel struct {
	repo *git.Repo
	db   meta.DB

	restackModel *sequencerui.RestackModel

	quitWithConflict bool
	err              error
}

func (vm *stackReparentViewModel) Init() tea.Cmd {
	state, err := vm.createState()
	if err != nil {
		return func() tea.Msg { return err }
	}
	vm.restackModel = sequencerui.NewRestackModel(vm.repo, vm.db)
	vm.restackModel.State = state
	return vm.restackModel.Init()
}

func (vm *stackReparentViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (vm *stackReparentViewModel) View() string {
	sb := strings.Builder{}
	sb.WriteString("Reparenting onto " + stackReparentFlags.Parent + "...\n")
	sb.WriteString(vm.restackModel.View())
	if vm.err != nil {
		sb.WriteString(vm.err.Error() + "\n")
	}
	return sb.String()
}

func (vm *stackReparentViewModel) writeState(state *sequencerui.RestackState) error {
	if state == nil {
		return vm.repo.WriteStateFile(git.StateFileKindRestack, nil)
	}
	return vm.repo.WriteStateFile(git.StateFileKindRestack, state)
}

func (vm *stackReparentViewModel) createState() (*sequencerui.RestackState, error) {
	currentBranch, err := vm.repo.CurrentBranchName()
	if err != nil {
		return nil, err
	}
	if isCurrentBranchTrunk, err := vm.repo.IsTrunkBranch(currentBranch); err != nil {
		return nil, err
	} else if isCurrentBranchTrunk {
		return nil, errors.New("current branch is a trunk branch")
	}
	if _, exist := vm.db.ReadTx().Branch(currentBranch); !exist {
		return nil, errors.New("current branch is not adopted to av")
	}

	if isParentBranchTrunk, err := vm.repo.IsTrunkBranch(stackReparentFlags.Parent); err != nil {
		return nil, err
	} else if !isParentBranchTrunk {
		if _, exist := vm.db.ReadTx().Branch(stackReparentFlags.Parent); !exist {
			return nil, errors.New("parent branch is not adopted to av")
		}
	}
	var state sequencerui.RestackState
	state.InitialBranch = currentBranch
	state.RelatedBranches = []string{currentBranch, stackReparentFlags.Parent}
	ops, err := planner.PlanForReparent(vm.db.ReadTx(), vm.repo, plumbing.NewBranchReferenceName(currentBranch), plumbing.NewBranchReferenceName(stackReparentFlags.Parent))
	if err != nil {
		return nil, err
	}
	if len(ops) == 0 {
		return nil, nothingToRestackError
	}
	state.Seq = sequencer.NewSequencer("origin", vm.db, ops)
	return &state, nil
}

func init() {
	stackReparentCmd.Flags().StringVar(
		&stackReparentFlags.Parent, "parent", "",
		"new parent branch name",
	)

	_ = stackReparentCmd.RegisterFlagCompletionFunc("parent", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		branches, _ := allBranches()
		return branches, cobra.ShellCompDirectiveDefault
	})
}
