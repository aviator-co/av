package gitui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/errutils"
	"github.com/aviator-co/av/internal/utils/uiutils"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/erikgeiser/promptkit/selection"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

const (
	continueDeletion = "Yes. Delete these merged branches."
	abortDeletion    = "No. Do not delete the merged branches."

	reasonNoPullRequest     = "PR not found."
	reasonHasChild          = "PR is already merged, but still have a child."
	reasonPRHeadNotFound    = "PR is already merged, but we cannot find which commit is merged."
	reasonPRHeadIsDifferent = "PR is already merged, but the local branch points to a different commit than the merged commit."
)

type deleteCandidate struct {
	branch plumbing.ReferenceName
	commit plumbing.Hash
}

type noDeleteBranch struct {
	branch plumbing.ReferenceName
	reason string
}

func NewPruneBranchModel(
	repo *git.Repo,
	db meta.DB,
	pruneFlag string,
	targetBranches []plumbing.ReferenceName,
	initialBranch string,
	onDone func() tea.Cmd,
) *PruneBranchModel {
	return &PruneBranchModel{
		repo:           repo,
		db:             db,
		pruneFlag:      pruneFlag,
		targetBranches: targetBranches,
		initialBranch:  initialBranch,
		spinner:        spinner.New(spinner.WithSpinner(spinner.Dot)),
		help:           help.New(),
		chooseNoPrune:  pruneFlag == "no",
		onDone:         onDone,
	}
}

type PruneBranchModel struct {
	repo           *git.Repo
	db             meta.DB
	pruneFlag      string
	targetBranches []plumbing.ReferenceName
	initialBranch  string
	spinner        spinner.Model
	help           help.Model
	onDone         func() tea.Cmd

	chooseNoPrune    bool
	deleteCandidates []deleteCandidate
	noDeleteBranches []noDeleteBranch
	deletePrompt     *selection.Model[string]

	calculatingCandidates bool
	askingForConfirmation bool
	runningDeletion       bool
	done                  bool
}

type PruneBranchProgress struct {
	candidateCalculationDone bool
	deletionDone             bool
}

func (vm *PruneBranchModel) Init() tea.Cmd {
	vm.calculatingCandidates = true
	return tea.Batch(vm.spinner.Tick, vm.calculateMergedBranches)
}

func (vm *PruneBranchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case *PruneBranchProgress:
		if msg.candidateCalculationDone {
			vm.calculatingCandidates = false
			if len(vm.deleteCandidates) == 0 || vm.chooseNoPrune {
				vm.done = true
				return vm, vm.onDone()
			}
			if vm.pruneFlag == "yes" {
				vm.runningDeletion = true
				return vm, vm.runDelete
			}
			vm.askingForConfirmation = true
			vm.deletePrompt = uiutils.NewLegacyPromptModel("Are you OK with deleting these merged branches?", []string{continueDeletion, abortDeletion})
			return vm, vm.deletePrompt.Init()
		}
		if msg.deletionDone {
			vm.runningDeletion = false
			vm.done = true
			return vm, vm.onDone()
		}
	case tea.KeyMsg:
		if vm.askingForConfirmation {
			switch msg.String() {
			case "enter":
				c, err := vm.deletePrompt.Value()
				if err != nil {
					return vm, uiutils.ErrCmd(err)
				}
				vm.askingForConfirmation = false
				vm.deletePrompt = nil
				if c != continueDeletion {
					vm.chooseNoPrune = true
					vm.done = true
					return vm, vm.onDone()
				}
				vm.runningDeletion = true
				return vm, vm.runDelete
			case "ctrl+c":
				return vm, tea.Quit
			default:
				_, cmd := vm.deletePrompt.Update(msg)
				return vm, cmd
			}
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		vm.spinner, cmd = vm.spinner.Update(msg)
		return vm, cmd
	}
	return vm, nil
}

func (vm *PruneBranchModel) View() string {
	if vm.calculatingCandidates {
		return colors.ProgressStyle.Render(vm.spinner.View() + "Finding the changed branches...\n")
	}

	sb := strings.Builder{}
	if len(vm.deleteCandidates) == 0 {
		sb.WriteString(colors.SuccessStyle.Render("✓ No merged branches to delete\n"))
	} else if vm.askingForConfirmation {
		sb.WriteString("Confirming the deletion of merged branches\n")
	} else if vm.runningDeletion {
		sb.WriteString(colors.ProgressStyle.Render(vm.spinner.View() + "Deleting merged branches...\n"))
	} else if vm.done {
		if vm.chooseNoPrune {
			sb.WriteString(colors.SuccessStyle.Render("✓ Not deleting merged branches\n"))
		} else {
			sb.WriteString(colors.SuccessStyle.Render("✓ Deleted the merged branches\n"))
		}
	}

	if len(vm.noDeleteBranches) > 0 {
		sb.WriteString("\n")
		sb.WriteString("  Following merged branches will be kept.\n")
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().MarginLeft(4).Render(vm.viewNoDeleteBranches()))
	}
	if len(vm.deleteCandidates) > 0 {
		sb.WriteString("\n")
		if vm.runningDeletion {
			sb.WriteString("  Following merged branches are being deleted ...\n")
		} else if vm.done && !vm.chooseNoPrune {
			sb.WriteString("  Following merged branches are deleted.\n")
		} else {
			sb.WriteString("  Following merged branches can be deleted.\n")
		}
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().MarginLeft(4).Render(vm.viewCandidates()))
	}

	if vm.deletePrompt != nil {
		sb.WriteString("\n")
		sb.WriteString(vm.deletePrompt.View())
		sb.WriteString(vm.help.ShortHelpView(uiutils.PromptKeys))
	}
	return sb.String()
}

func (vm *PruneBranchModel) viewCandidates() string {
	sb := strings.Builder{}
	for _, branch := range vm.deleteCandidates {
		sb.WriteString(fmt.Sprintf("%s: %s\n", branch.branch.Short(), branch.commit.String()))
	}
	return sb.String()
}

func (vm *PruneBranchModel) viewNoDeleteBranches() string {
	sb := strings.Builder{}
	for _, branch := range vm.noDeleteBranches {
		sb.WriteString(branch.branch.Short() + ": " + branch.reason + "\n")
	}
	return sb.String()
}

func (vm *PruneBranchModel) runDelete() tea.Msg {
	// Checkout the detached HEAD so that we can delete the branches. We cannot delete the
	// branches that are checked out.
	if err := vm.repo.Detach(context.Background()); err != nil {
		return err
	}

	// Always restore the checked out state, even if deletion fails.
	// Use a variable to capture any deletion error and restore HEAD before returning.
	var deletionErr error
	var worktreeBranches []string

	// Delete in the reverse order just in case. The targetBranches are sorted in the parent ->
	// child order.
	for i := len(vm.deleteCandidates) - 1; i >= 0; i-- {
		branch := vm.deleteCandidates[i]
		if err := vm.repo.BranchDelete(context.Background(), branch.branch.Short()); err != nil {
			// Check if the error is due to the branch being checked out in a worktree
			if exiterr, ok := errutils.As[*exec.ExitError](err); ok &&
				strings.Contains(string(exiterr.Stderr), "used by worktree") {
				// Collect worktree branches but continue deleting others
				worktreeBranches = append(worktreeBranches, branch.branch.Short())
				continue
			} else {
				// Other errors are fatal
				deletionErr = errors.Errorf("cannot delete merged branch %q: %v", branch.branch.Short(), err)
				break
			}
		}
		tx := vm.db.WriteTx()
		tx.DeleteBranch(branch.branch.Short())
		if err := tx.Commit(); err != nil {
			deletionErr = err
			break
		}
	}

	// Restore the checked out state before returning any error.
	if err := vm.CheckoutInitialState(); err != nil {
		// If we have both a deletion error and a checkout error, prioritize the checkout error
		// as it's more critical (leaves user in a bad state).
		if deletionErr != nil {
			return errors.Errorf("failed to restore branch after deletion error: %v (original error: %v)", err, deletionErr)
		}
		return errors.Errorf("failed to restore branch: %v", err)
	}

	// Build worktree error message if any branches couldn't be deleted
	var worktreeErr error
	if len(worktreeBranches) > 0 {
		var sb strings.Builder
		sb.WriteString("Could not delete the following branches (checked out in worktrees):\n")
		for _, br := range worktreeBranches {
			fmt.Fprintf(&sb, "- %s\n", br)
		}
		sb.WriteString("Use 'git worktree list' to see all worktrees\n")
		sb.WriteString("Remove worktrees with 'git worktree remove <path>' or checkout a different branch in those worktrees\n")
		worktreeErr = errors.New(sb.String())
	}

	// Return both fatal deletion errors and worktree warnings
	if deletionErr != nil {
		if worktreeErr != nil {
			return errors.Errorf("fatal error during branch deletion: %v\n\n%v", deletionErr, worktreeErr)
		}
		return deletionErr
	}

	if worktreeErr != nil {
		return worktreeErr
	}

	return &PruneBranchProgress{deletionDone: true}
}

func (vm *PruneBranchModel) CheckoutInitialState() error {
	if vm.initialBranch != "" {
		initialHead, err := vm.repo.GoGitRepo().
			Reference(plumbing.NewBranchReferenceName(vm.initialBranch), true)
		if err == nil {
			if initialHead.Type() == plumbing.HashReference {
				// Normal reference that points to a commit. Checking out.
				if _, err := vm.repo.CheckoutBranch(context.Background(), &git.CheckoutBranch{Name: initialHead.Name().Short()}); err != nil {
					return err
				}
				return nil
			}
		} else if err != plumbing.ErrReferenceNotFound {
			return err
		}
	}

	// The branch is deleted. Let's checkout the default branch.
	defaultBranch := vm.repo.DefaultBranch()
	defaultBranchRef := plumbing.NewBranchReferenceName(defaultBranch)
	ref, err := vm.repo.GoGitRepo().Reference(defaultBranchRef, true)
	if err == nil {
		if _, err := vm.repo.CheckoutBranch(context.Background(), &git.CheckoutBranch{Name: ref.Name().Short()}); err != nil {
			return err
		}
		return nil
	}

	// The default branch doesn't exist. Check the remote tracking branch.
	remote, err := vm.repo.GoGitRepo().Remote(vm.repo.GetRemoteName())
	if err != nil {
		return errors.Errorf("failed to get remote %s: %v", vm.repo.GetRemoteName(), err)
	}
	remoteConfig := remote.Config()
	rtb := mapToRemoteTrackingBranch(remoteConfig, defaultBranchRef)
	if rtb != nil {
		ref, err = vm.repo.GoGitRepo().Reference(*rtb, true)
		if err == nil {
			if _, err := vm.repo.CheckoutBranch(context.Background(), &git.CheckoutBranch{Name: ref.Hash().String()}); err != nil {
				return err
			}
			return nil
		}
	}

	// No remote tracking branch. Skip.
	return nil
}

func (vm *PruneBranchModel) calculateMergedBranches() tea.Msg {
	remoteBranches, err := vm.repo.LsRemote(context.Background(), vm.repo.GetRemoteName())
	if err != nil {
		return err
	}
	var noDeleteBranches []noDeleteBranch
	var deleteCandidates []deleteCandidate
	for _, br := range vm.targetBranches {
		avbr, _ := vm.db.ReadTx().Branch(br.Short())
		if avbr.MergeCommit == "" {
			continue
		}
		if vm.hasOpenChildren(br) {
			noDeleteBranches = append(
				noDeleteBranches,
				noDeleteBranch{branch: br, reason: reasonHasChild},
			)
			continue
		}
		if avbr.PullRequest == nil {
			noDeleteBranches = append(
				noDeleteBranches,
				noDeleteBranch{branch: br, reason: reasonNoPullRequest},
			)
			continue
		}
		remoteHash, ok := remoteBranches[fmt.Sprintf("refs/pull/%d/head", avbr.PullRequest.Number)]
		if !ok {
			noDeleteBranches = append(
				noDeleteBranches,
				noDeleteBranch{branch: br, reason: reasonPRHeadNotFound},
			)
			continue
		}
		ref, err := vm.repo.GoGitRepo().Reference(br, true)
		if err != nil {
			return err
		}
		if ref.Hash().String() != remoteHash {
			noDeleteBranches = append(
				noDeleteBranches,
				noDeleteBranch{branch: br, reason: reasonPRHeadIsDifferent},
			)
			continue
		}
		deleteCandidates = append(deleteCandidates, deleteCandidate{branch: br, commit: ref.Hash()})
	}
	vm.noDeleteBranches = noDeleteBranches
	vm.deleteCandidates = deleteCandidates
	return &PruneBranchProgress{candidateCalculationDone: true}
}

func (vm *PruneBranchModel) hasOpenChildren(br plumbing.ReferenceName) bool {
	for _, child := range meta.SubsequentBranches(vm.db.ReadTx(), br.Short()) {
		childBr, _ := vm.db.ReadTx().Branch(child)
		if childBr.MergeCommit == "" {
			return true
		}
	}
	return false
}

func mapToRemoteTrackingBranch(
	remoteConfig *config.RemoteConfig,
	refName plumbing.ReferenceName,
) *plumbing.ReferenceName {
	for _, fetch := range remoteConfig.Fetch {
		if fetch.Match(refName) {
			dst := fetch.Dst(refName)
			return &dst
		}
	}
	return nil
}
