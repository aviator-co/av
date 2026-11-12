package gitui

import (
	"context"
	"fmt"
	"strings"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/colors"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-git/go-git/v5/plumbing"
)

// FastForwardBranchesModel is a Bubbletea model that fast-forwards each local
// branch in targetBranches whose remote tracking branch is strictly ahead of
// the local branch. This lets users adopt commits that someone else pushed on
// top of their branch.
type FastForwardBranchesModel struct {
	repo           *git.Repo
	targetBranches []plumbing.ReferenceName
	onDone         func() tea.Cmd

	ffBranches []string
	done       bool
}

type ffBranchesDone struct{}

func NewFastForwardBranchesModel(
	repo *git.Repo,
	targetBranches []plumbing.ReferenceName,
	onDone func() tea.Cmd,
) *FastForwardBranchesModel {
	return &FastForwardBranchesModel{
		repo:           repo,
		targetBranches: targetBranches,
		onDone:         onDone,
	}
}

func (m *FastForwardBranchesModel) Init() tea.Cmd {
	return m.run
}

func (m *FastForwardBranchesModel) run() tea.Msg {
	ctx := context.Background()
	remote := m.repo.GetRemoteName()

	currentBranch, err := m.repo.CurrentBranchName()
	if err != nil {
		currentBranch = ""
	}

	for _, br := range m.targetBranches {
		name := br.Short()
		localRef := br.String()
		remoteRef := fmt.Sprintf("refs/remotes/%s/%s", remote, name)
		remoteExists, err := m.repo.DoesRefExist(ctx, remoteRef)
		if err != nil || !remoteExists {
			continue
		}

		// Skip if local is not an ancestor of remote (diverged or local is ahead).
		isBehind, err := m.repo.IsAncestor(ctx, localRef, remoteRef)
		if err != nil || !isBehind {
			continue
		}
		// Skip if remote is also an ancestor of local, meaning they point at
		// the same commit and there is nothing to fast-forward.
		isAhead, err := m.repo.IsAncestor(ctx, remoteRef, localRef)
		if err != nil || isAhead {
			continue
		}

		if currentBranch == name {
			if _, err := m.repo.Run(ctx, &git.RunOpts{
				Args:      []string{"merge", "--ff-only", remoteRef},
				ExitError: true,
			}); err != nil {
				continue
			}
		} else {
			if err := m.repo.UpdateRef(ctx, &git.UpdateRef{
				Ref: localRef,
				New: remoteRef,
			}); err != nil {
				continue
			}
		}
		m.ffBranches = append(m.ffBranches, name)
	}
	m.done = true
	return ffBranchesDone{}
}

func (m *FastForwardBranchesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case ffBranchesDone:
		return m, m.onDone()
	}
	return m, nil
}

func (m *FastForwardBranchesModel) View() string {
	if !m.done || len(m.ffBranches) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, name := range m.ffBranches {
		sb.WriteString(colors.SuccessStyle.Render(fmt.Sprintf("✓ Fast-forwarded %s to match remote", name)) + "\n")
	}
	return sb.String()
}
