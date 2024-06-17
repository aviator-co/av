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
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var stackRestackFlags struct {
	All      bool
	Current  bool
	Abort    bool
	Continue bool
	Skip     bool
	DryRun   bool
}

var stackRestackCmd = &cobra.Command{
	Use:   "restack",
	Short: "Restack branches",
	Args:  cobra.NoArgs,
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
		p := tea.NewProgram(&stackRestackViewModel{repo: repo, db: db}, opts...)
		model, err := p.Run()
		if err != nil {
			return err
		}
		if err := model.(*stackRestackViewModel).err; err != nil {
			if errors.Is(err, nothingToRestackError) {
				return nil
			}
			return actions.ErrExitSilently{ExitCode: 1}
		}
		if model.(*stackRestackViewModel).quitWithConflict {
			return actions.ErrExitSilently{ExitCode: 1}
		}
		return nil
	},
}

type stackRestackViewModel struct {
	repo *git.Repo
	db   meta.DB

	restackModel *sequencerui.RestackModel

	quitWithConflict bool
	err              error
}

func (vm *stackRestackViewModel) Init() tea.Cmd {
	state, err := vm.readState()
	if err != nil {
		return func() tea.Msg { return err }
	}
	if state == nil {
		if stackRestackFlags.Abort || stackRestackFlags.Continue || stackRestackFlags.Skip {
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
	vm.restackModel.Abort = stackRestackFlags.Abort
	vm.restackModel.Continue = stackRestackFlags.Continue
	vm.restackModel.Skip = stackRestackFlags.Skip
	vm.restackModel.DryRun = stackRestackFlags.DryRun
	return vm.restackModel.Init()
}

func (vm *stackRestackViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (vm *stackRestackViewModel) View() string {
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

func (vm *stackRestackViewModel) readState() (*sequencerui.RestackState, error) {
	var state sequencerui.RestackState
	if err := vm.repo.ReadStateFile(git.StateFileKindRestack, &state); err != nil && os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &state, nil
}

func (vm *stackRestackViewModel) writeState(state *sequencerui.RestackState) error {
	if state == nil {
		return vm.repo.WriteStateFile(git.StateFileKindRestack, nil)
	}
	return vm.repo.WriteStateFile(git.StateFileKindRestack, state)
}

func (vm *stackRestackViewModel) createState() (*sequencerui.RestackState, error) {
	var state sequencerui.RestackState
	var currentBranch string
	if dh, err := vm.repo.DetachedHead(); err != nil {
		return nil, err
	} else if !dh {
		currentBranch, err = vm.repo.CurrentBranchName()
		if err != nil {
			return nil, err
		}
	}
	state.InitialBranch = currentBranch

	if stackRestackFlags.All {
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

	ops, err := planner.PlanForRestack(vm.db.ReadTx(), vm.repo, currentBranchRef, stackRestackFlags.All, stackRestackFlags.Current)
	if err != nil {
		return nil, err
	}
	state.Seq = sequencer.NewSequencer(vm.repo.GetRemoteName(), vm.db, ops)
	return &state, nil
}

func init() {
	stackRestackCmd.Flags().BoolVar(
		&stackRestackFlags.All, "all", false,
		"restack all branches",
	)
	stackRestackCmd.Flags().BoolVar(
		&stackRestackFlags.Current, "current", false,
		"only restack up to the current branch\n(don't recurse into descendant branches)",
	)
	stackRestackCmd.Flags().BoolVar(
		&stackRestackFlags.Continue, "continue", false,
		"continue an in-progress restack",
	)
	stackRestackCmd.Flags().BoolVar(
		&stackRestackFlags.Abort, "abort", false,
		"abort an in-progress restack",
	)
	stackRestackCmd.Flags().BoolVar(
		&stackRestackFlags.Skip, "skip", false,
		"skip the current commit and continue an in-progress restack",
	)
	stackRestackCmd.Flags().BoolVar(
		&stackRestackFlags.DryRun, "dry-run", false,
		"dry-run the restack",
	)

	stackRestackCmd.MarkFlagsMutuallyExclusive("continue", "abort", "skip")
}
