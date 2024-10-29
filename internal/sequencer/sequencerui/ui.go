package sequencerui

import (
	"fmt"
	"strings"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/sequencer"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-git/go-git/v5/plumbing"
)

func NewRestackModel(repo *git.Repo, db meta.DB) *RestackModel {
	return &RestackModel{
		repo:    repo,
		db:      db,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		Command: "av stack restack",
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
	Skip       bool
	Continue   bool
	Abort      bool
	DryRun     bool
	Autosquash bool
	State      *RestackState
	Command    string

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
	if vm.Skip || vm.Continue || vm.Abort ||vm.Autosquash {
		if vm.Abort {
			vm.abortedBranch = vm.State.Seq.CurrentSyncRef
		} else if vm.Autosquash {
			fmt.Print("Do you want to autosquash these commits? [y/n]: ")
			var response string
			fmt.Scanln(&response)
			response = strings.ToLower(strings.TrimSpace(response))
		
			switch response {
			case "y", "yes":
				fmt.Println("Autosquash enabled.")
			case "n", "no":
				fmt.Println("Autosquash disabled.")
			default:
				fmt.Println("Invalid response. Autosquash aborted.")
			}
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
		if vm.State.Seq.CurrentSyncRef != "" && !vm.Autosquash {
			sb.WriteString(
				colors.ProgressStyle.Render(
					vm.spinner.View() + "Restacking " + vm.State.Seq.CurrentSyncRef.Short() + "...",
				),
			)
		} else if vm.abortedBranch != "" {
			sb.WriteString(colors.FailureStyle.Render("✗ Restack is aborted"))
		} else if (!vm.Autosquash) {
			sb.WriteString(colors.SuccessStyle.Render("✓ Restack is done"))
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
			nodes = stackutils.BuildStackTreeAllBranches(
				vm.db.ReadTx(),
				vm.State.InitialBranch,
				true,
			)
		} else if (!vm.Autosquash) {
			nodes, err = stackutils.BuildStackTreeRelatedBranchStacks(vm.db.ReadTx(), vm.State.InitialBranch, true, vm.State.RelatedBranches)
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
					if bn == vm.State.Seq.CurrentSyncRef {
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
				vm.State.Seq.CurrentSyncRef.Short(),
			) + "\n",
		)
		sb.WriteString(vm.rebaseConflictErrorHeadline + "\n")
		sb.WriteString(vm.rebaseConflictHint + "\n")
		sb.WriteString("\n")
		sb.WriteString(
			"Resolve the conflicts and continue the restack with " + colors.CliCmd(
				vm.Command+" --continue",
			),
		)
	}
	return sb.String()
}

func (vm *RestackModel) runSeqWithContinuationFlags() tea.Msg {

	result, err := vm.State.Seq.Run(vm.repo, vm.db, vm.Abort, vm.Continue, vm.Skip,vm.Autosquash)
	return &RestackProgress{result: result, err: err}
}

func (vm *RestackModel) runSeq() tea.Msg {

	result, err := vm.State.Seq.Run(vm.repo, vm.db, false, false, false, false)
	return &RestackProgress{result: result, err: err}
}
