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
		p := tea.NewProgram(stackReparentViewModel{
			repo:    repo,
			db:      db,
			spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		}, opts...)
		model, err := p.Run()
		if err != nil {
			return err
		}
		if err := model.(stackReparentViewModel).err; err != nil {
			return actions.ErrExitSilently{ExitCode: 1}
		}
		if s := model.(stackReparentViewModel).rebaseConflictErrorHeadline; s != "" {
			return actions.ErrExitSilently{ExitCode: 1}
		}
		return nil
	},
}

type stackReparentState struct {
	InitialBranch   string
	NewParentBranch string
	Seq             *sequencer.Sequencer
}

type stackReparentSeqResult struct {
	result *git.RebaseResult
	err    error
}

type stackReparentViewModel struct {
	repo    *git.Repo
	db      meta.DB
	state   *stackReparentState
	spinner spinner.Model

	rebaseConflictErrorHeadline string
	rebaseConflictHint          string
	abortedBranch               plumbing.ReferenceName
	err                         error
}

func (vm stackReparentViewModel) Init() tea.Cmd {
	return tea.Batch(vm.spinner.Tick, vm.initCmd)
}

func (vm stackReparentViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case error:
		vm.err = msg
		return vm, tea.Quit
	case *stackReparentState:
		vm.state = msg
		if stackReparentFlags.Skip || stackReparentFlags.Continue || stackReparentFlags.Abort {
			if stackReparentFlags.Abort {
				vm.abortedBranch = vm.state.Seq.CurrentSyncRef
			}
			return vm, vm.runSeqWithContinuationFlags
		}
		return vm, vm.runSeq
	case *stackReparentSeqResult:
		if msg.err == nil && msg.result == nil {
			// Finished the sequence.
			if err := vm.repo.WriteStateFile(git.StateFileKindReparent, nil); err != nil {
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
			if err := vm.repo.WriteStateFile(git.StateFileKindReparent, vm.state); err != nil {
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

func (vm stackReparentViewModel) View() string {
	sb := strings.Builder{}
	if vm.state != nil && vm.state.Seq != nil {
		sb.WriteString("Reparenting " + vm.state.InitialBranch + " onto " + vm.state.NewParentBranch + "...\n")
		if vm.state.Seq.CurrentSyncRef != "" {
			sb.WriteString("Restacking " + vm.state.Seq.CurrentSyncRef.Short() + "...\n")
		} else if vm.abortedBranch != "" {
			sb.WriteString("Restack aborted\n")
		} else {
			sb.WriteString("Restack done\n")
		}
		// The sequencer operates from top to bottom. The branches that are synced before
		// the current branches are already synced. The branches that come after the current
		// branch are pending.
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

		nodes, err := stackutils.BuildStackTreeRelatedBranchStacks(vm.db.ReadTx(), vm.state.InitialBranch, true, []string{vm.state.InitialBranch, vm.state.NewParentBranch})
		if err != nil {
			sb.WriteString("Failed to build stack tree: " + err.Error() + "\n")
		} else {
			for _, node := range nodes {
				sb.WriteString(stackutils.RenderTree(node, func(branchName string, isTrunk bool) string {
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
		}
	}
	if vm.rebaseConflictErrorHeadline != "" {
		sb.WriteString("\n")
		sb.WriteString(colors.Failure("Rebase conflict while rebasing ", vm.state.Seq.CurrentSyncRef.Short()) + "\n")
		sb.WriteString(vm.rebaseConflictErrorHeadline + "\n")
		sb.WriteString(vm.rebaseConflictHint + "\n")
		sb.WriteString("\n")
		sb.WriteString("Resolve the conflicts and continue the reparent with " + colors.CliCmd("av stack reparent --continue") + "\n")
	}
	if vm.err != nil {
		sb.WriteString(vm.err.Error() + "\n")
	}
	return sb.String()
}

func (vm stackReparentViewModel) initCmd() tea.Msg {
	var state stackReparentState
	if err := vm.repo.ReadStateFile(git.StateFileKindReparent, &state); err != nil && os.IsNotExist(err) {
		currentBranch, err := vm.repo.CurrentBranchName()
		if err != nil {
			return err
		}
		if isCurrentBranchTrunk, err := vm.repo.IsTrunkBranch(currentBranch); err != nil {
			return err
		} else if isCurrentBranchTrunk {
			return errors.New("current branch is a trunk branch")
		}
		if _, exist := vm.db.ReadTx().Branch(currentBranch); !exist {
			return errors.New("current branch is not adopted to av")
		}

		if isParentBranchTrunk, err := vm.repo.IsTrunkBranch(stackReparentFlags.Parent); err != nil {
			return err
		} else if !isParentBranchTrunk {
			if _, exist := vm.db.ReadTx().Branch(stackReparentFlags.Parent); !exist {
				return errors.New("parent branch is not adopted to av")
			}
		}
		state.InitialBranch = currentBranch
		state.NewParentBranch = stackReparentFlags.Parent
		ops, err := planner.PlanForReparent(vm.db.ReadTx(), vm.repo, plumbing.NewBranchReferenceName(currentBranch), plumbing.NewBranchReferenceName(stackReparentFlags.Parent))
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

func (vm stackReparentViewModel) runSeqWithContinuationFlags() tea.Msg {
	result, err := vm.state.Seq.Run(vm.repo, vm.db, stackReparentFlags.Abort, stackReparentFlags.Continue, stackReparentFlags.Skip)
	return &stackReparentSeqResult{result: result, err: err}
}

func (vm stackReparentViewModel) runSeq() tea.Msg {
	result, err := vm.state.Seq.Run(vm.repo, vm.db, false, false, false)
	return &stackReparentSeqResult{result: result, err: err}
}

func init() {
	stackReparentCmd.Flags().BoolVar(
		&stackReparentFlags.Continue, "continue", false,
		"continue an in-progress reparent",
	)
	stackReparentCmd.Flags().BoolVar(
		&stackReparentFlags.Abort, "abort", false,
		"abort an in-progress reparent",
	)
	stackReparentCmd.Flags().BoolVar(
		&stackReparentFlags.Skip, "skip", false,
		"skip the current commit and continue an in-progress reparent",
	)
	stackReparentCmd.Flags().StringVar(
		&stackReparentFlags.Parent, "parent", "",
		"new parent branch name",
	)

	stackReparentCmd.MarkFlagsMutuallyExclusive("continue", "abort", "skip", "parent")
	_ = stackReparentCmd.RegisterFlagCompletionFunc("parent", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		branches, _ := allBranches()
		return branches, cobra.ShellCompDirectiveDefault
	})
}
