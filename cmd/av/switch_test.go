package main

import (
	"fmt"
	"testing"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeNode(name string, children ...*stackutils.StackTreeNode) *stackutils.StackTreeNode {
	return &stackutils.StackTreeNode{
		Branch:   &stackutils.StackTreeBranchInfo{BranchName: name},
		Children: children,
	}
}

func branchNames(nodes []*stackutils.StackTreeNode) []string {
	var names []string
	for _, n := range nodes {
		names = append(names, n.Branch.BranchName)
	}
	return names
}

func TestPruneDeletedBranches(t *testing.T) {
	branches := map[string]*stackTreeBranchInfo{
		"main":    {BranchName: "main"},
		"alive":   {BranchName: "alive"},
		"deleted": {BranchName: "deleted", Deleted: true},
		"child1":  {BranchName: "child1"},
		"child2":  {BranchName: "child2"},
	}

	t.Run("no deleted branches", func(t *testing.T) {
		nodes := []*stackutils.StackTreeNode{
			makeNode("main", makeNode("alive")),
		}
		result := pruneDeletedBranches(nodes, branches)
		assert.Len(t, result, 1)
		assert.Equal(t, "main", result[0].Branch.BranchName)
		assert.Len(t, result[0].Children, 1)
	})

	t.Run("deleted leaf branch is removed", func(t *testing.T) {
		nodes := []*stackutils.StackTreeNode{
			makeNode("main", makeNode("alive"), makeNode("deleted")),
		}
		result := pruneDeletedBranches(nodes, branches)
		assert.Len(t, result, 1)
		assert.Equal(t, []string{"alive"}, branchNames(result[0].Children))
	})

	t.Run("deleted branch promotes children", func(t *testing.T) {
		nodes := []*stackutils.StackTreeNode{
			makeNode("main", makeNode("deleted", makeNode("child1"), makeNode("child2"))),
		}
		result := pruneDeletedBranches(nodes, branches)
		assert.Len(t, result, 1)
		assert.Equal(t, []string{"child1", "child2"}, branchNames(result[0].Children))
	})

	t.Run("deleted root promotes children", func(t *testing.T) {
		nodes := []*stackutils.StackTreeNode{
			makeNode("deleted", makeNode("child1")),
		}
		result := pruneDeletedBranches(nodes, branches)
		assert.Equal(t, []string{"child1"}, branchNames(result))
	})
}

func TestSwitchViewScrollsToChosenBranch(t *testing.T) {
	names := []string{"development", "bugfix/login-flow"}
	for i := 1; i <= 10; i++ {
		names = append(names, fmt.Sprintf("av-stack-%02d", i))
	}
	branches := map[string]*stackTreeBranchInfo{}
	var root *stackutils.StackTreeNode
	for i := len(names) - 1; i >= 0; i-- {
		branches[names[i]] = &stackTreeBranchInfo{BranchName: names[i]}
		if root == nil {
			root = makeNode(names[i])
		} else {
			root = makeNode(names[i], root)
		}
	}
	var branchList []*stackTreeBranchInfo
	for i := len(names) - 1; i >= 0; i-- {
		branchList = append(branchList, branches[names[i]])
	}

	const termHeight = 20
	var model tea.Model = switchViewModel{
		help:                help.New(),
		currentHEADBranch:   "av-stack-10",
		currentChosenBranch: "av-stack-10",
		rootNodes:           []*stackutils.StackTreeNode{root},
		branchList:          branchList,
		branches:            branches,
	}
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: termHeight})

	assertVisible := func(branch string) {
		t.Helper()
		view := model.View().Content
		require.LessOrEqual(t, lipgloss.Height(view), termHeight)
		assert.Contains(t, view, branch)
		assert.Contains(t, view, "Choose which branch to check out")
	}

	assertVisible("av-stack-10")
	for i := len(names) - 2; i >= 0; i-- {
		model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		assertVisible(names[i])
	}
	for i := 1; i < len(names); i++ {
		model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyUp})
		assertVisible(names[i])
	}
}

func TestSwitchViewEscCancels(t *testing.T) {
	var model tea.Model = switchViewModel{
		help:                help.New(),
		currentHEADBranch:   "main",
		currentChosenBranch: "main",
		rootNodes:           []*stackutils.StackTreeNode{makeNode("main")},
		branchList:          []*stackTreeBranchInfo{{BranchName: "main"}},
		branches:            map[string]*stackTreeBranchInfo{"main": {BranchName: "main"}},
	}
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd())
	assert.False(t, model.(switchViewModel).checkingOut)
}
