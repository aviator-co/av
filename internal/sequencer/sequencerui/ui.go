package sequencerui

import (
	"strings"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/sequencer"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-git/go-git/v5/plumbing"
)

func NewRestackModel(repo *git.Repo, db meta.DB) *RestackModel {
	return &RestackModel{
		repo:    repo,
		db:      db,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
	}
}

type RestackState struct {
	InitialBranch   string
	RestackingAll   bool
	RelatedBranches []string
	Seq             *sequencer.Sequencer
}

type RestackProgress struct {
	result *git.RebaseResult
	err    error
}

type RestackConflict struct{}
type RestackAbort struct{}
type RestackDone struct{}

type RestackModel struct {
	Skip     bool
	Continue bool
	Abort    bool
	DryRun   bool
	State    *RestackState

	repo *git.Repo
	db   meta.DB

	spinner                     spinner.Model
	rebaseConflictErrorHeadline string
	rebaseConflictHint          string
	abortedBranch               plumbing.ReferenceName
}

func (vm *RestackModel) Init() tea.Cmd {
	return tea.Batch(vm.spinner.Tick, vm.initCmd)
}

func (vm *RestackModel) initCmd() tea.Msg {
	if vm.Skip || vm.Continue || vm.Abort {
		if vm.Abort {
			vm.abortedBranch = vm.State.Seq.CurrentSyncRef
		}
		return vm.runSeqWithContinuationFlags()
	}
	return vm.runSeq()
}

func (vm *RestackModel) Update(msg tea.Msg) (*RestackModel, tea.Cmd) {
	switch msg := msg.(type) {
	case *RestackProgress:
		if msg.err == nil && msg.result == nil {
			// Finished the sequence.
			if vm.State.InitialBranch != "" {
				if _, err := vm.repo.CheckoutBranch(&git.CheckoutBranch{Name: vm.State.InitialBranch}); err != nil {
					return vm, func() tea.Msg { return err }
				}
			}
			if vm.abortedBranch != "" {
				return vm, func() tea.Msg { return &RestackAbort{} }
			}
			return vm, func() tea.Msg { return &RestackDone{} }
		}
		if msg.result != nil && msg.result.Status == git.RebaseConflict {
			vm.rebaseConflictErrorHeadline = msg.result.ErrorHeadline
			vm.rebaseConflictHint = msg.result.Hint
			return vm, func() tea.Msg { return &RestackConflict{} }
		}
		if msg.err != nil {
			return vm, func() tea.Msg { return msg.err }
		}
		return vm, vm.runSeq
	case spinner.TickMsg:
		var cmd tea.Cmd
		vm.spinner, cmd = vm.spinner.Update(msg)
		return vm, cmd
	}
	return vm, nil
}

func (vm *RestackModel) View() string {
	sb := strings.Builder{}
	if vm.State != nil && vm.State.Seq != nil {
		if vm.State.Seq.CurrentSyncRef != "" {
			sb.WriteString("Restacking " + vm.State.Seq.CurrentSyncRef.Short() + "...\n")
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
		for _, op := range vm.State.Seq.Operations {
			if op.Name == vm.State.Seq.CurrentSyncRef || op.Name == vm.abortedBranch {
				seenCurrent = true
			} else if !seenCurrent {
				syncedBranches[op.Name] = true
			} else {
				pendingBranches[op.Name] = true
			}
		}

		var nodes []*stackutils.StackTreeNode
		var err error
		if vm.State.RestackingAll {
			nodes = stackutils.BuildStackTreeAllBranches(vm.db.ReadTx(), vm.State.InitialBranch, true)
		} else {
			nodes, err = stackutils.BuildStackTreeRelatedBranchStacks(vm.db.ReadTx(), vm.State.InitialBranch, true, vm.State.RelatedBranches)
		}
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
					if bn == vm.State.Seq.CurrentSyncRef {
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
		sb.WriteString(colors.Failure("Rebase conflict while rebasing ", vm.State.Seq.CurrentSyncRef.Short()) + "\n")
		sb.WriteString(vm.rebaseConflictErrorHeadline + "\n")
		sb.WriteString(vm.rebaseConflictHint + "\n")
		sb.WriteString("\n")
		sb.WriteString("Resolve the conflicts and continue the restack with " + colors.CliCmd("av stack restack --continue") + "\n")
	}
	return sb.String()
}

func (vm *RestackModel) runSeqWithContinuationFlags() tea.Msg {
	result, err := vm.State.Seq.Run(vm.repo, vm.db, vm.Abort, vm.Continue, vm.Skip)
	return &RestackProgress{result: result, err: err}
}

func (vm *RestackModel) runSeq() tea.Msg {
	result, err := vm.State.Seq.Run(vm.repo, vm.db, false, false, false)
	return &RestackProgress{result: result, err: err}
}
