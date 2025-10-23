package actions

import (
	"context"

	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/treedetector"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/aviator-co/av/internal/utils/uiutils"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-git/go-git/v5/plumbing"
)

func NewFindAdoptableLocalBranchesModel(
	repo *git.Repo,
	db meta.DB,
	onDone func(
		branches map[plumbing.ReferenceName]*treedetector.BranchPiece,
		rootNodes []*stackutils.StackTreeNode,
		adoptionTargets []plumbing.ReferenceName,
	) tea.Cmd,
) tea.Model {
	return &FindAdoptableLocalBranchesModel{
		repo:    repo,
		db:      db,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		onDone:  onDone,
	}
}

type FindAdoptableLocalBranchesModel struct {
	repo    *git.Repo
	db      meta.DB
	spinner spinner.Model
	done    bool
	onDone  func(
		branches map[plumbing.ReferenceName]*treedetector.BranchPiece,
		rootNodes []*stackutils.StackTreeNode,
		adoptionTargets []plumbing.ReferenceName,
	) tea.Cmd
}

func (m *FindAdoptableLocalBranchesModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		unmanagedBranches, err := m.getUnmanagedBranches()
		if err != nil {
			return err
		}
		branches, err := treedetector.DetectBranches(context.Background(), m.repo, unmanagedBranches)
		if err != nil {
			return err
		}
		if len(branches) == 0 {
			m.done = true
			return uiutils.SimpleCommandMsg{
				Cmd: m.onDone(nil, nil, nil),
			}
		}
		rootNodes := treedetector.ConvertToStackTree(m.db, branches, plumbing.HEAD, false)
		if len(rootNodes) == 0 {
			m.done = true
			return uiutils.SimpleCommandMsg{
				Cmd: m.onDone(nil, nil, nil),
			}
		}
		adoptionTargets := m.getAdoptionTargets(rootNodes[0])
		m.done = true
		return uiutils.SimpleCommandMsg{
			Cmd: m.onDone(branches, rootNodes, adoptionTargets),
		}
	})
}

func (m *FindAdoptableLocalBranchesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *FindAdoptableLocalBranchesModel) View() string {
	if m.done {
		return ""
	}
	return colors.ProgressStyle.Render(m.spinner.View() + "Finding adoptable branches...")
}

func (m *FindAdoptableLocalBranchesModel) getUnmanagedBranches() ([]plumbing.ReferenceName, error) {
	tx := m.db.ReadTx()
	adoptedBranches := tx.AllBranches()
	branches, err := m.repo.GoGitRepo().Branches()
	if err != nil {
		return nil, err
	}
	var ret []plumbing.ReferenceName
	if err := branches.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference {
			return nil
		}
		if _, adopted := adoptedBranches[ref.Name().Short()]; !adopted {
			ret = append(ret, ref.Name())
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (m *FindAdoptableLocalBranchesModel) getAdoptionTargets(node *stackutils.StackTreeNode) []plumbing.ReferenceName {
	var ret []plumbing.ReferenceName
	for _, child := range node.Children {
		ret = append(ret, m.getAdoptionTargets(child)...)
	}
	if node.Branch.ParentBranchName != "" {
		_, adopted := m.db.ReadTx().Branch(node.Branch.BranchName)
		if !adopted {
			ret = append(ret, plumbing.NewBranchReferenceName(node.Branch.BranchName))
		}
	}
	return ret
}
