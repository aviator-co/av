package actions

import (
	"strings"

	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/uiutils"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type AdoptingBranch struct {
	Name        string
	Parent      meta.BranchState
	PullRequest *meta.PullRequest
}

func NewAdoptBranchesModel(
	db meta.DB,
	branches []AdoptingBranch,
	onDone func() tea.Cmd,
) tea.Model {
	return &AdoptBranchesModel{
		db:       db,
		spinner:  spinner.New(spinner.WithSpinner(spinner.Dot)),
		branches: branches,
		onDone:   onDone,
	}
}

type AdoptBranchesModel struct {
	db       meta.DB
	spinner  spinner.Model
	branches []AdoptingBranch
	done     bool
	onDone   func() tea.Cmd
}

func (m *AdoptBranchesModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.adoptBranches)
}

func (m *AdoptBranchesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *AdoptBranchesModel) View() string {
	var ss []string
	if m.done {
		ss = append(ss, colors.SuccessStyle.Render("✓ Adoption complete"))
	} else {
		ss = append(ss, colors.ProgressStyle.Render(m.spinner.View()+"Adopting the chosen branches..."))
	}
	ss = append(ss, "")
	for _, branch := range m.branches {
		ss = append(ss, "  * "+branch.Name+" → "+branch.Parent.Name)
	}
	return strings.Join(ss, "\n")
}

func (m *AdoptBranchesModel) adoptBranches() tea.Msg {
	tx := m.db.WriteTx()
	for _, branch := range m.branches {
		bi, _ := tx.Branch(branch.Name)
		bi.Parent = branch.Parent
		bi.PullRequest = branch.PullRequest
		tx.SetBranch(bi)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	m.done = true
	return uiutils.SimpleCommandMsg{Cmd: m.onDone()}
}
