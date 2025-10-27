package actions

import (
	"slices"
	"strings"

	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-git/go-git/v5/plumbing"
)

type BranchTreeInfo struct {
	TitleLine string
	Body      string
}

func NewAdoptTreeSelectorModel(
	infos map[plumbing.ReferenceName]BranchTreeInfo,
	rootNodes []*stackutils.StackTreeNode,
	adoptionTargets []plumbing.ReferenceName,
	currentHEADBranch plumbing.ReferenceName,
	onDone func(chosenTargets []plumbing.ReferenceName) tea.Cmd,
) tea.Model {
	chosenTargets := make(map[plumbing.ReferenceName]bool)
	for _, branch := range adoptionTargets {
		// By default choose everything.
		chosenTargets[branch] = true
	}

	return &AdoptTreeSelectorModel{
		help:              help.New(),
		infos:             infos,
		rootNodes:         rootNodes,
		adoptionTargets:   adoptionTargets,
		currentHEADBranch: currentHEADBranch,
		currentCursor:     adoptionTargets[0],
		chosenTargets:     chosenTargets,
		onDone:            onDone,
	}
}

type AdoptTreeSelectorModel struct {
	help              help.Model
	infos             map[plumbing.ReferenceName]BranchTreeInfo
	rootNodes         []*stackutils.StackTreeNode
	adoptionTargets   []plumbing.ReferenceName
	currentHEADBranch plumbing.ReferenceName
	onDone            func(chosenTargets []plumbing.ReferenceName) tea.Cmd

	done          bool
	currentCursor plumbing.ReferenceName
	chosenTargets map[plumbing.ReferenceName]bool
}

func (m *AdoptTreeSelectorModel) Init() tea.Cmd {
	return func() tea.Msg {
		return nil
	}
}

func (m *AdoptTreeSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "up", "k", "ctrl+p":
			m.currentCursor = m.getPreviousBranch()
		case "down", "j", "ctrl+n":
			m.currentCursor = m.getNextBranch()
		case " ":
			m.toggleAdoption(m.currentCursor)
		case "enter":
			m.done = true
			var chosenTargets []plumbing.ReferenceName
			for bn := range m.chosenTargets {
				chosenTargets = append(chosenTargets, bn)
			}
			return m, m.onDone(chosenTargets)
		}
	}
	return m, nil
}

func (m *AdoptTreeSelectorModel) View() string {
	var ss []string
	if m.done {
		ss = append(ss, colors.SuccessStyle.Render("✓ Choose which branches to adopt"))
	} else {
		ss = append(ss, colors.QuestionStyle.Render("? Choose which branches to adopt"))
	}
	for _, rootNode := range m.rootNodes {
		ss = append(ss, "")
		ss = append(
			ss,
			stackutils.RenderTree(
				rootNode,
				func(branchName string, isTrunk bool) string {
					bn := plumbing.NewBranchReferenceName(branchName)
					out := m.renderBranch(bn, isTrunk)
					if !m.done && bn == m.currentCursor {
						out = colors.PromptChoice.Render(out)
					}
					return out
				},
			),
		)
	}
	ss = append(ss, "")
	if !m.done {
		ss = append(ss, m.help.ShortHelpView(promptKeys))
	}
	return strings.Join(ss, "\n")
}

func (m *AdoptTreeSelectorModel) renderBranch(branch plumbing.ReferenceName, isTrunk bool) string {
	if isTrunk {
		return branch.Short()
	}
	info := m.infos[branch]

	var lines []string
	if !slices.Contains(m.adoptionTargets, branch) {
		lines = append(lines, info.TitleLine)
	} else if m.chosenTargets[branch] {
		lines = append(lines, "[✓] "+info.TitleLine)
	} else {
		lines = append(lines, "[ ] "+info.TitleLine)
	}
	if info.Body != "" {
		lines = append(lines, lipgloss.NewStyle().MarginLeft(4).Render(info.Body))
	}
	return strings.Join(lines, "\n")
}

func (m *AdoptTreeSelectorModel) getPreviousBranch() plumbing.ReferenceName {
	for i, branch := range m.adoptionTargets {
		if branch == m.currentCursor {
			if i == 0 {
				return m.currentCursor
			}
			return m.adoptionTargets[i-1]
		}
	}
	return m.currentCursor
}

func (m *AdoptTreeSelectorModel) getNextBranch() plumbing.ReferenceName {
	for i, branch := range m.adoptionTargets {
		if branch == m.currentCursor {
			if i == len(m.adoptionTargets)-1 {
				return m.currentCursor
			}
			return m.adoptionTargets[i+1]
		}
	}
	return m.currentCursor
}

func (m *AdoptTreeSelectorModel) toggleAdoption(branch plumbing.ReferenceName) {
	if m.chosenTargets[branch] {
		// Going to unchoose. Unchoose all children as well.
		delete(m.chosenTargets, branch)
		for _, rootNode := range m.rootNodes {
			descendantNames := stackutils.GetDescendantBranchNames(rootNode, branch.Short())
			for _, descName := range descendantNames {
				descBN := plumbing.NewBranchReferenceName(descName)
				delete(m.chosenTargets, descBN)
			}
		}
	} else {
		// Going to choose. Choose all parents as well.
		m.chosenTargets[branch] = true
		for _, rootNode := range m.rootNodes {
			parentNames := stackutils.GetParentBranchNames(rootNode, branch.Short())
			for _, parentName := range parentNames {
				parentBN := plumbing.NewBranchReferenceName(parentName)
				if slices.Contains(m.adoptionTargets, parentBN) {
					m.chosenTargets[parentBN] = true
				}
			}
		}
	}
}

var promptKeys = []key.Binding{
	key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	key.NewBinding(
		key.WithKeys("space"),
		key.WithHelp("space", "select / unselect"),
	),
	key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "adopt selected branches"),
	),
	key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "cancel"),
	),
}
