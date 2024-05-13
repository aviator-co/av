package actions_test

import (
	"testing"

	"github.com/aviator-co/av/internal/actions"
	"github.com/aviator-co/av/internal/meta"
	"github.com/aviator-co/av/internal/utils/maputils"
	"github.com/aviator-co/av/internal/utils/stackutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadPRMetadata(t *testing.T) {
	tx := fakeReadTx{}
	prMeta := actions.PRMetadata{
		Parent:     "foo",
		ParentHead: "bar",
		ParentPull: 123,
		Trunk:      "baz",
	}
	prBody := actions.AddPRMetadataAndStack("Hello! This is a cool PR that does some neat things.", prMeta, "foo", nil, tx)
	prMeta2, err := actions.ReadPRMetadata(prBody)
	require.NoError(t, err)
	assert.Equal(t, prMeta.Parent, prMeta2.Parent)
	assert.Equal(t, prMeta.ParentHead, prMeta2.ParentHead)
	assert.Equal(t, prMeta.ParentPull, prMeta2.ParentPull)
	assert.Equal(t, prMeta.Trunk, prMeta2.Trunk)

	prBody = actions.AddPRMetadataAndStack(prBody, actions.PRMetadata{
		Parent:     "foo2",
		ParentHead: "bar2",
		ParentPull: 1234,
		Trunk:      "baz2",
	}, "foo2", nil, tx)
	assert.Contains(t, prBody, "Hello! This is a cool PR that does some neat things.\n\n")
	prMeta2, err = actions.ReadPRMetadata(prBody)
	require.NoError(t, err)
	assert.Equal(t, "foo2", prMeta2.Parent)
	assert.Equal(t, "bar2", prMeta2.ParentHead)
}

func TestPRMetadataPreservesBody(t *testing.T) {
	tx := fakeReadTx{}
	sampleMeta := actions.PRMetadata{
		Parent:     "foo",
		ParentHead: "bar",
		ParentPull: 123,
		Trunk:      "baz",
	}
	body1 := actions.AddPRMetadataAndStack(
		"Hello! This is a cool PR that does some neat things.",
		sampleMeta,
		"foo",
		nil,
		tx,
	)
	// Add some text to the end of the body (as if someone had edited manually)
	body1 += "\n\nIt's very neat, actually."

	body2 := actions.AddPRMetadataAndStack(body1, sampleMeta, "foo", nil, tx)
	assert.Contains(t, body2, "Hello! This is a cool PR that does some neat things.")
	assert.Contains(t, body2, "It's very neat, actually.")
	assert.Contains(t, body2, "\n"+actions.PRMetadataCommentStart)
}

func TestPRWithStack(t *testing.T) {
	tx := fakeReadTx{
		"baz": {
			Name: "baz",
			Parent: meta.BranchState{
				Name:  "main",
				Trunk: true,
			},
			PullRequest: &meta.PullRequest{
				Number:    1001,
				Permalink: "https://github.com/org/repo/pull/1001",
			},
		},
		"foo": {
			Name: "foo",
			Parent: meta.BranchState{
				Name: "baz",
			},
			PullRequest: &meta.PullRequest{
				Number:    1002,
				Permalink: "https://github.com/org/repo/pull/1002",
			},
		},
	}
	stack := &stackutils.StackTreeNode{
		Branch: &stackutils.StackTreeBranchInfo{
			BranchName: "main",
		},
		Children: []*stackutils.StackTreeNode{
			{
				Branch: &stackutils.StackTreeBranchInfo{
					BranchName: "baz",
				},
				Children: []*stackutils.StackTreeNode{
					{
						Branch: &stackutils.StackTreeBranchInfo{
							BranchName: "foo",
						},
						Children: []*stackutils.StackTreeNode{},
					},
				},
			},
		},
	}

	sampleMeta := actions.PRMetadata{
		Parent:     "foo",
		ParentHead: "bar",
		ParentPull: 123,
		Trunk:      "baz",
	}
	body1 := actions.AddPRMetadataAndStack(
		"Hello! This is a cool PR that does some neat things.",
		sampleMeta,
		"foo",
		stack,
		tx,
	)

	assert.Equal(t, `<!-- av pr stack begin -->
<table><tr><td><details><summary><b>Depends on #1001.</b> This PR is part of a stack created with <a href="https://github.com/aviator-co/av">Aviator</a>.</summary>

* ➡️ **#1002**
* **#1001**
* `+"`"+`main`+"`"+`
</details></td></tr></table>
<!-- av pr stack end -->

Hello! This is a cool PR that does some neat things.

<!-- av pr metadata
This information is embedded by the av CLI when creating PRs to track the status of stacks when using Aviator. Please do not delete or edit this section of the PR.
`+"```"+`
{"parent":"foo","parentHead":"bar","parentPull":123,"trunk":"baz"}
`+"```"+`
-->
`, body1)
}

func TestPRWithForkedStack(t *testing.T) {
	tx := fakeReadTx{
		"baz": {
			Name: "baz",
			Parent: meta.BranchState{
				Name:  "main",
				Trunk: true,
			},
			PullRequest: &meta.PullRequest{
				Number:    1001,
				Permalink: "https://github.com/org/repo/pull/1001",
			},
		},
		"foo": {
			Name: "foo",
			Parent: meta.BranchState{
				Name: "baz",
			},
			PullRequest: &meta.PullRequest{
				Number:    1002,
				Permalink: "https://github.com/org/repo/pull/1002",
			},
		},
		"qux": {
			Name: "qux",
			Parent: meta.BranchState{
				Name:  "main",
				Trunk: true,
			},
			PullRequest: &meta.PullRequest{
				Number:    1003,
				Permalink: "https://github.com/org/repo/pull/1003",
			},
		},
	}
	stack := &stackutils.StackTreeNode{
		Branch: &stackutils.StackTreeBranchInfo{
			BranchName: "main",
		},
		Children: []*stackutils.StackTreeNode{
			{
				Branch: &stackutils.StackTreeBranchInfo{
					BranchName: "baz",
				},
				Children: []*stackutils.StackTreeNode{
					{
						Branch: &stackutils.StackTreeBranchInfo{
							BranchName: "foo",
						},
						Children: []*stackutils.StackTreeNode{},
					},
				},
			},
			{
				Branch: &stackutils.StackTreeBranchInfo{
					BranchName: "qux",
				},
				Children: []*stackutils.StackTreeNode{},
			},
		},
	}

	sampleMeta := actions.PRMetadata{
		Parent:     "foo",
		ParentHead: "bar",
		ParentPull: 123,
		Trunk:      "baz",
	}
	body1 := actions.AddPRMetadataAndStack(
		"Hello! This is a cool PR that does some neat things.",
		sampleMeta,
		"foo",
		stack,
		tx,
	)

	assert.Equal(t, `<!-- av pr stack begin -->
<table><tr><td><details><summary><b>Depends on #1001.</b> This PR is part of a stack created with <a href="https://github.com/aviator-co/av">Aviator</a>.</summary>

* `+"`"+`main`+"`"+`
  * **#1001**
    * ➡️ **#1002**
  * **#1003**
</details></td></tr></table>
<!-- av pr stack end -->

Hello! This is a cool PR that does some neat things.

<!-- av pr metadata
This information is embedded by the av CLI when creating PRs to track the status of stacks when using Aviator. Please do not delete or edit this section of the PR.
`+"```"+`
{"parent":"foo","parentHead":"bar","parentPull":123,"trunk":"baz"}
`+"```"+`
-->
`, body1)
}

type fakeReadTx map[string]meta.Branch

func (tx fakeReadTx) Repository() (meta.Repository, bool) {
	return meta.Repository{}, false
}

func (tx fakeReadTx) Branch(name string) (meta.Branch, bool) {
	branch, ok := tx[name]
	if branch.Name == "" {
		branch.Name = name
	}
	return branch, ok
}

func (tx fakeReadTx) AllBranches() map[string]meta.Branch {
	return maputils.Copy(tx)
}
