package actions

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/uiutils"
)

type GitFetchModel struct {
	repo     *git.Repo
	spinner  spinner.Model
	refspecs []string
	onDone   func() tea.Cmd

	done   bool
	failed bool
	stdout bytes.Buffer
	stderr bytes.Buffer
}

func NewGitFetchModel(
	repo *git.Repo,
	refspecs []string,
	onDone func() tea.Cmd,
) tea.Model {
	return &GitFetchModel{
		repo:     repo,
		spinner:  spinner.New(spinner.WithSpinner(spinner.Dot)),
		refspecs: refspecs,
		onDone:   onDone,
	}
}

func (m *GitFetchModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetchBranches)
}

func (m *GitFetchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *GitFetchModel) View() tea.View {
	var lines []string
	if m.done {
		if m.failed {
			lines = append(lines, colors.FailureStyle.Render("✗ Failed to fetch branches"))
		} else {
			lines = append(lines, colors.SuccessStyle.Render("✓ Fetched branches successfully"))
		}
	} else {
		lines = append(lines, colors.ProgressStyle.Render(m.spinner.View()+" Fetching branches..."))
	}
	if out := m.stdout.String(); out != "" {
		lines = append(lines, "", lipgloss.NewStyle().MarginLeft(4).Render(out))
	}
	if out := m.stderr.String(); out != "" {
		lines = append(lines, "", lipgloss.NewStyle().MarginLeft(4).Render(out))
	}
	return tea.NewView(strings.Join(lines, "\n"))
}

func (m *GitFetchModel) fetchBranches() tea.Msg {
	args := []string{
		"-C", m.repo.Dir(),
		"fetch",
		m.repo.GetRemoteName(),
	}
	args = append(args, m.refspecs...)
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Stdout = &m.stdout
	cmd.Stderr = &m.stderr
	if err := cmd.Run(); err != nil {
		m.done = true
		m.failed = true
		return err
	}
	m.done = true
	return uiutils.SimpleCommandMsg{Cmd: m.onDone()}
}
