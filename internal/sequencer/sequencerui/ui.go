package sequencerui

import (
	"context"
	"strings"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/sequencer"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/aviator-co/av/internal/utils/uiutils"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-git/go-git/v5/plumbing"
)

type RestackStateOptions struct {
	Skip       bool
	Continue   bool
	Abort      bool
	DryRun     bool
	Command    string
	OnConflict func() tea.Cmd
	OnAbort    func() tea.Cmd
	OnDone     func() tea.Cmd
}

func NewRestackModel(
	repo *git.Repo,
	db meta.DB,
	state *RestackState,
	options RestackStateOptions,
) *RestackModel {
	if options.Command == "" {
		options.Command = "av restack"
	}
	return &RestackModel{
		repo:    repo,
		db:      db,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		state:   state,
		options: options,
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

type RestackModel struct {
	repo    *git.Repo
	db      meta.DB
	state   *RestackState
	options RestackStateOptions

	spinner                     spinner.Model
	rebaseConflictErrorHeadline string
	rebaseConflictHint          string
	abortedBranch               plumbing.ReferenceName
}

func (vm *RestackModel) Init() tea.Cmd {
	return tea.Batch(vm.spinner.Tick, vm.initCmd)
}

func (vm *RestackModel) initCmd() tea.Msg {
	if vm.options.Skip || vm.options.Continue || vm.options.Abort {
		if vm.options.Abort {
			vm.abortedBranch = vm.state.Seq.CurrentSyncRef
		}
		return vm.runSeqWithContinuationFlags()
	}
	return vm.runSeq()
}

func (vm *RestackModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case *RestackProgress:
		if msg.err == nil && msg.result == nil {
			// Finished the sequence.
			if vm.state.InitialBranch != "" {
				if _, err := vm.repo.CheckoutBranch(context.Background(), &git.CheckoutBranch{Name: vm.state.InitialBranch}); err != nil {
					return vm, uiutils.ErrCmd(err)
				}
			}
			if vm.abortedBranch != "" {
				return vm, vm.options.OnAbort()
			}
			return vm, vm.options.OnDone()
		}
		if msg.result != nil && msg.result.Status == git.RebaseConflict {
			vm.rebaseConflictErrorHeadline = msg.result.ErrorHeadline
			vm.rebaseConflictHint = msg.result.Hint
			return vm, vm.options.OnConflict()
		}
		if msg.err != nil {
			return vm, uiutils.ErrCmd(msg.err)
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
	if vm.state != nil && vm.state.Seq != nil {
		if vm.state.Seq.CurrentSyncRef != "" {
			sb.WriteString(
				colors.ProgressStyle.Render(
					vm.spinner.View() + "Restacking " + vm.state.Seq.CurrentSyncRef.Short() + "...",
				),
			)
		} else if vm.abortedBranch != "" {
			sb.WriteString(colors.FailureStyle.Render("✗ Restack is aborted"))
		} else {
			sb.WriteString(colors.SuccessStyle.Render("✓ Restack is done"))
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

		var nodes []*stackutils.StackTreeNode
		var err error
		if vm.state.RestackingAll {
			nodes = stackutils.BuildStackTreeAllBranches(
				vm.db.ReadTx(),
				vm.state.InitialBranch,
				true,
			)
		} else {
			nodes, err = stackutils.BuildStackTreeRelatedBranchStacks(vm.db.ReadTx(), vm.state.InitialBranch, true, vm.state.RelatedBranches)
		}
		if err != nil {
			sb.WriteString("\n")
			sb.WriteString("Failed to build stack tree: " + err.Error())
		} else if len(nodes) > 0 {
			sb.WriteString("\n")
			sb.WriteString("\n")
			for _, node := range nodes {
				sb.WriteString(stackutils.RenderTree(node, func(branchName string, isTrunk bool) string {
					var suffix string
					avbr, _ := vm.db.ReadTx().Branch(branchName)
					if avbr.MergeCommit != "" {
						suffix += " (merged)"
					}
					hash, err := vm.repo.GoGitRepo().ResolveRevision(plumbing.Revision(branchName))
					if err == nil && hash != nil {
						suffix += " " + hash.String()[:7]
					}

					bn := plumbing.NewBranchReferenceName(branchName)
					if syncedBranches[bn] {
						return colors.SuccessStyle.Render("✓ " + branchName + suffix)
					}
					if pendingBranches[bn] {
						return colors.ProgressStyle.Render(branchName + suffix)
					}
					if bn == vm.state.Seq.CurrentSyncRef {
						return colors.ProgressStyle.Render(vm.spinner.View() + branchName + suffix)
					}
					if bn == vm.abortedBranch {
						return colors.FailureStyle.Render("✗ " + branchName + suffix)
					}
					return branchName + suffix
				}))
			}
			sb.WriteString("\n")
		}
	}
	if vm.rebaseConflictErrorHeadline != "" {
		sb.WriteString("\n")
		sb.WriteString(
			colors.FailureStyle.Render(
				"Rebase conflict while rebasing ",
				vm.state.Seq.CurrentSyncRef.Short(),
			) + "\n",
		)
		sb.WriteString(vm.rebaseConflictErrorHeadline + "\n")
		sb.WriteString(vm.rebaseConflictHint + "\n")
		sb.WriteString("\n")
		sb.WriteString(
			"Resolve the conflicts and continue the restack with " + colors.CliCmd(
				vm.options.Command+" --continue",
			),
		)
	}
	return sb.String()
}

func (vm *RestackModel) runSeqWithContinuationFlags() tea.Msg {
	result, err := vm.state.Seq.Run(
		context.Background(),
		vm.repo,
		vm.db,
		vm.options.Abort,
		vm.options.Continue,
		vm.options.Skip,
	)
	return &RestackProgress{result: result, err: err}
}

func (vm *RestackModel) runSeq() tea.Msg {
	result, err := vm.state.Seq.Run(context.Background(), vm.repo, vm.db, false, false, false)
	return &RestackProgress{result: result, err: err}
}
