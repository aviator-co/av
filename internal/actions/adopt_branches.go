package actions

import (
	"strings"

	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/treedetector"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/uiutils"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-git/go-git/v5/plumbing"
)

func NewAdoptBranchesModel(
	db meta.DB,
	chosenTargets []plumbing.ReferenceName,
	branches map[plumbing.ReferenceName]*treedetector.BranchPiece,
	onDone func() tea.Cmd,
) tea.Model {
	return &AdoptBranchesModel{
		db:            db,
		spinner:       spinner.New(spinner.WithSpinner(spinner.Dot)),
		chosenTargets: chosenTargets,
		branches:      branches,
		onDone:        onDone,
	}
}

type AdoptBranchesModel struct {
	db            meta.DB
	spinner       spinner.Model
	chosenTargets []plumbing.ReferenceName
	branches      map[plumbing.ReferenceName]*treedetector.BranchPiece
	done          bool
	onDone        func() tea.Cmd
}

func (m *AdoptBranchesModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.adoptBranches)
}

func (m *AdoptBranchesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	for _, branch := range m.chosenTargets {
		piece := m.branches[branch]
		ss = append(ss, "  * "+branch.Short()+" → "+piece.Parent.Short())
		for _, c := range piece.IncludedCommits {
			title, _, _ := strings.Cut(c.Message, "\n")
			ss = append(ss, "    * "+title)
		}
	}
	return strings.Join(ss, "\n")
}

func (m *AdoptBranchesModel) adoptBranches() tea.Msg {
	tx := m.db.WriteTx()
	for _, branch := range m.chosenTargets {
		piece := m.branches[branch]
		bi, _ := tx.Branch(branch.Short())
		bi.Parent = meta.BranchState{
			Name:  piece.Parent.Short(),
			Trunk: piece.ParentIsTrunk,
		}
		if !piece.ParentIsTrunk {
			bi.Parent.Head = piece.ParentMergeBase.String()
		}
		tx.SetBranch(bi)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	m.done = true
	return uiutils.SimpleCommandMsg{Cmd: m.onDone()}
}
