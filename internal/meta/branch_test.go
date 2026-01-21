package meta

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testReadTx struct {
	branches map[string]Branch
}

func (tx testReadTx) Repository() Repository {
	return Repository{}
}

func (tx testReadTx) Branch(name string) (Branch, bool) {
	branch, ok := tx.branches[name]
	if ok && branch.Name == "" {
		branch.Name = name
	}
	return branch, ok
}

func (tx testReadTx) AllBranches() map[string]Branch {
	return tx.branches
}

func TestValidateNoCycle(t *testing.T) {
	for _, tt := range []struct {
		name      string
		tx        ReadTx
		branch    string
		parent    BranchState
		expectErr bool
	}{
		{
			name:   "trunk parent allowed",
			tx:     testReadTx{branches: map[string]Branch{}},
			branch: "feature",
			parent: BranchState{Name: "main", Trunk: true},
		},
		{
			name:      "missing parent rejected",
			tx:        testReadTx{branches: map[string]Branch{}},
			branch:    "feature",
			parent:    BranchState{Name: "missing", Trunk: false},
			expectErr: true,
		},
		{
			name:      "self parent rejected",
			tx:        testReadTx{branches: map[string]Branch{}},
			branch:    "feature",
			parent:    BranchState{Name: "feature", Trunk: false},
			expectErr: true,
		},
		{
			name: "cycle in parent chain rejected",
			tx: testReadTx{branches: map[string]Branch{
				"b": {Name: "b", Parent: BranchState{Name: "c", Trunk: false}},
				"c": {Name: "c", Parent: BranchState{Name: "a", Trunk: false}},
				"a": {Name: "a", Parent: BranchState{Name: "main", Trunk: true}},
			}},
			branch:    "a",
			parent:    BranchState{Name: "b", Trunk: false},
			expectErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNoCycle(tt.tx, tt.branch, tt.parent)
			if tt.expectErr {
				assert.Error(t, err)
				if tt.name == "missing parent rejected" {
					assert.Contains(t, err.Error(), "missing from av metadata")
				}
				if tt.name == "cycle in parent chain rejected" || tt.name == "self parent rejected" {
					assert.Contains(t, err.Error(), "cyclical branch dependencies")
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
