package main

import (
	"testing"

	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/stretchr/testify/assert"
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
