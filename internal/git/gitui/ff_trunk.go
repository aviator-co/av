package gitui

import (
	"context"
	"fmt"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/colors"
	tea "github.com/charmbracelet/bubbletea"
)

// FastForwardTrunkModel is a Bubbletea model that fast-forward merges the local
// trunk branch (e.g., main/master) to match its remote tracking branch.
type FastForwardTrunkModel struct {
	repo   *git.Repo
	onDone func() tea.Cmd

	done     bool
	skipped  bool
	diverged bool
	trunk    string
}

type ffTrunkDone struct{}

func NewFastForwardTrunkModel(
	repo *git.Repo,
	onDone func() tea.Cmd,
) *FastForwardTrunkModel {
	return &FastForwardTrunkModel{
		repo:   repo,
		onDone: onDone,
	}
}

func (m *FastForwardTrunkModel) Init() tea.Cmd {
	return m.run
}

func (m *FastForwardTrunkModel) run() tea.Msg {
	ctx := context.Background()
	m.trunk = m.repo.DefaultBranch()
	remote := m.repo.GetRemoteName()

	// Check if the local trunk branch exists.
	exists, err := m.repo.DoesBranchExist(ctx, m.trunk)
	if err != nil || !exists {
		m.skipped = true
		return ffTrunkDone{}
	}

	// Check if the remote tracking branch exists.
	remoteExists, err := m.repo.DoesRemoteBranchExist(ctx, m.trunk)
	if err != nil || !remoteExists {
		m.skipped = true
		return ffTrunkDone{}
	}

	// Check if we are currently on the trunk branch. If so, we need to use
	// "git merge --ff-only" directly. Otherwise, we can use update-ref to
	// update the local ref without checking it out.
	currentBranch, err := m.repo.CurrentBranchName()
	if err != nil {
		// Detached HEAD or other issue; just try the update-ref approach.
		currentBranch = ""
	}

	remoteRef := fmt.Sprintf("%s/%s", remote, m.trunk)

	if currentBranch == m.trunk {
		// We're on the trunk branch, use merge --ff-only.
		_, err := m.repo.Run(ctx, &git.RunOpts{
			Args:      []string{"merge", "--ff-only", remoteRef},
			ExitError: true,
		})
		if err != nil {
			m.diverged = true
			return ffTrunkDone{}
		}
	} else {
		// Not on the trunk branch. Verify that the remote is a fast-forward
		// of the local branch using merge-base --is-ancestor, then update the
		// ref.
		_, err := m.repo.Run(ctx, &git.RunOpts{
			Args:      []string{"merge-base", "--is-ancestor", fmt.Sprintf("refs/heads/%s", m.trunk), fmt.Sprintf("refs/remotes/%s/%s", remote, m.trunk)},
			ExitError: true,
		})
		if err != nil {
			m.diverged = true
			return ffTrunkDone{}
		}
		_, err = m.repo.Run(ctx, &git.RunOpts{
			Args:      []string{"update-ref", fmt.Sprintf("refs/heads/%s", m.trunk), fmt.Sprintf("refs/remotes/%s/%s", remote, m.trunk)},
			ExitError: true,
		})
		if err != nil {
			m.diverged = true
			return ffTrunkDone{}
		}
	}

	m.done = true
	return ffTrunkDone{}
}

func (m *FastForwardTrunkModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case ffTrunkDone:
		return m, m.onDone()
	}
	return m, nil
}

func (m *FastForwardTrunkModel) View() string {
	if m.skipped {
		return ""
	}
	if m.diverged {
		return colors.ProgressStyle.Render(fmt.Sprintf("  Could not fast-forward %s (local branch has diverged from remote)", m.trunk)) + "\n"
	}
	if m.done {
		return colors.SuccessStyle.Render(fmt.Sprintf("✓ Fast-forwarded %s to match remote", m.trunk)) + "\n"
	}
	return ""
}
