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
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var stackRestackFlags struct {
	DryRun   bool
	Abort    bool
	Continue bool
	Skip     bool
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
		p := tea.NewProgram(stackRestackViewModel{
			repo:    repo,
			db:      db,
			spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		}, opts...)
		model, err := p.Run()
		if err != nil {
			return err
		}
		if err := model.(stackRestackViewModel).err; err != nil {
			return actions.ErrExitSilently{ExitCode: 1}
		}
		if s := model.(stackRestackViewModel).rebaseConflictErrorHeadline; s != "" {
			return actions.ErrExitSilently{ExitCode: 1}
		}
		return nil
	},
}

type stackRestackState struct {
	InitialBranch string
	StNode        *stackutils.StackTreeNode
	Seq           *sequencer.Sequencer
}

type stackRestackSeqResult struct {
	result *git.RebaseResult
	err    error
}

type stackRestackViewModel struct {
	repo    *git.Repo
	db      meta.DB
	state   *stackRestackState
	spinner spinner.Model

	rebaseConflictErrorHeadline string
	rebaseConflictHint          string
	abortedBranch               plumbing.ReferenceName
	err                         error
}

func (vm stackRestackViewModel) Init() tea.Cmd {
	return tea.Batch(vm.spinner.Tick, vm.initCmd)
}

func (vm stackRestackViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case error:
		vm.err = msg
		return vm, tea.Quit
	case *stackRestackState:
		vm.state = msg
		if stackRestackFlags.DryRun {
			return vm, tea.Quit
		}
		if stackRestackFlags.Skip || stackRestackFlags.Continue || stackRestackFlags.Abort {
			if stackRestackFlags.Abort {
				vm.abortedBranch = vm.state.Seq.CurrentSyncRef
			}
			return vm, vm.runSeqWithContinuationFlags
		}
		return vm, vm.runSeq
	case *stackRestackSeqResult:
		if msg.err == nil && msg.result == nil {
			// Finished the sequence.
			if err := vm.repo.WriteStateFile(git.StateFileKindRestack, nil); err != nil {
				vm.err = err
			}
			if _, err := vm.repo.CheckoutBranch(&git.CheckoutBranch{Name: vm.state.InitialBranch}); err != nil {
				vm.err = err
			}
			return vm, tea.Quit
		}
		if msg.result != nil && msg.result.Status == git.RebaseConflict {
			vm.rebaseConflictErrorHeadline = msg.result.ErrorHeadline
			vm.rebaseConflictHint = msg.result.Hint
			if err := vm.repo.WriteStateFile(git.StateFileKindRestack, vm.state); err != nil {
				vm.err = err
			}
			return vm, tea.Quit
		}
		vm.err = msg.err
		if vm.err != nil {
			return vm, tea.Quit
		}
		return vm, vm.runSeq
	case spinner.TickMsg:
		var cmd tea.Cmd
		vm.spinner, cmd = vm.spinner.Update(msg)
		return vm, cmd
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return vm, tea.Quit
		}
	}
	return vm, nil
}

func (vm stackRestackViewModel) View() string {
	sb := strings.Builder{}
	if vm.state != nil && vm.state.Seq != nil {
		if vm.state.Seq.CurrentSyncRef != "" {
			sb.WriteString("Restacking " + vm.state.Seq.CurrentSyncRef.Short() + "...\n")
		} else if vm.abortedBranch != "" {
			sb.WriteString("Restack aborted\n")
		} else {
			sb.WriteString("Restack done\n")
		}
		syncedBranches := map[plumbing.ReferenceName]bool{}
		pendingBranches := map[plumbing.ReferenceName]bool{}
		seenCurrent := false
		for _, op := range vm.state.Seq.Operations {
			if op.Name == vm.state.Seq.CurrentSyncRef || op.Name == vm.abortedBranch {
				seenCurrent = true
			} else if !seenCurrent {
				syncedBranches[op.Name] = true
			} else {
				pendingBranches[op.Name] = true
			}
		}

		sb.WriteString(stackutils.RenderTree(vm.state.StNode, func(branchName string, isTrunk bool) string {
			bn := plumbing.NewBranchReferenceName(branchName)
			if syncedBranches[bn] {
				return colors.Success("✓ " + branchName)
			}
			if pendingBranches[bn] {
				return lipgloss.NewStyle().Foreground(colors.Amber500).Render(branchName)
			}
			if bn == vm.state.Seq.CurrentSyncRef {
				return lipgloss.NewStyle().Foreground(colors.Amber500).Render(vm.spinner.View() + branchName)
			}
			if bn == vm.abortedBranch {
				return colors.Failure("✗ " + branchName)
			}
			return branchName
		}))
	}
	if vm.rebaseConflictErrorHeadline != "" {
		sb.WriteString("\n")
		sb.WriteString(colors.Failure("Rebase conflict while rebasing ", vm.state.Seq.CurrentSyncRef.Short()) + "\n")
		sb.WriteString(vm.rebaseConflictErrorHeadline + "\n")
		sb.WriteString(vm.rebaseConflictHint + "\n")
		sb.WriteString("\n")
		sb.WriteString("Resolve the conflicts and continue the restack with " + colors.CliCmd("av stack restack --continue") + "\n")
	}
	if vm.err != nil {
		sb.WriteString(vm.err.Error() + "\n")
	}
	return sb.String()
}

func (vm stackRestackViewModel) initCmd() tea.Msg {
	var state stackRestackState
	if err := vm.repo.ReadStateFile(git.StateFileKindRestack, &state); err != nil && os.IsNotExist(err) {
		var currentBranch string
		if dh, err := vm.repo.DetachedHead(); err != nil {
			return err
		} else if !dh {
			currentBranch, err = vm.repo.CurrentBranchName()
			if err != nil {
				return err
			}
		}
		if _, exist := vm.db.ReadTx().Branch(currentBranch); !exist {
			return errors.New("current branch is not adopted to av")
		}
		state.InitialBranch = currentBranch
		state.StNode, err = stackutils.BuildStackTreeCurrentStack(vm.db.ReadTx(), currentBranch, true)
		if err != nil {
			return err
		}
		targetBranches, err := planner.GetTargetBranches(vm.db.ReadTx(), vm.repo, false, planner.CurrentStack)
		if err != nil {
			return err
		}
		ops, err := planner.PlanForRestack(vm.db.ReadTx(), vm.repo, targetBranches)
		if err != nil {
			return err
		}
		if len(ops) == 0 {
			return errors.New("nothing to restack")
		}
		state.Seq = sequencer.NewSequencer("origin", vm.db, ops)
	} else if err != nil {
		return err
	}
	return &state
}

func (vm stackRestackViewModel) runSeqWithContinuationFlags() tea.Msg {
	result, err := vm.state.Seq.Run(vm.repo, vm.db, stackRestackFlags.Abort, stackRestackFlags.Continue, stackRestackFlags.Skip)
	return &stackRestackSeqResult{result: result, err: err}
}

func (vm stackRestackViewModel) runSeq() tea.Msg {
	result, err := vm.state.Seq.Run(vm.repo, vm.db, false, false, false)
	return &stackRestackSeqResult{result: result, err: err}
}

func init() {
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
