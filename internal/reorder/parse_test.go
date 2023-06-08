package reorder

import (
	"reflect"
	"testing"
)

func TestParseCmd(t *testing.T) {
	for _, tt := range []struct {
		Input string
		Cmd   Cmd
		Err   bool
	}{
		{"stack-branch", StackBranchCmd{}, true},
		{"stack-branch feature-one", StackBranchCmd{Name: "feature-one"}, false},
		{"stack-branch feature-one --parent master", StackBranchCmd{Name: "feature-one", Parent: "master"}, false},
		{"stack-branch feature-one --trunk master", StackBranchCmd{Name: "feature-one", Trunk: "master"}, false},
		{"stack-branch feature-one --parent master --trunk master", StackBranchCmd{}, true},
		{"pick", PickCmd{}, true},
		{"pick foo", PickCmd{Commit: "foo"}, false},
		{"pick foo bar", PickCmd{}, true},
		{"delete-branch", DeleteBranchCmd{}, true},
		{"delete-branch foo", DeleteBranchCmd{Name: "foo"}, false},
		{"delete-branch foo bar", DeleteBranchCmd{}, true},
		{"db foo --delete-ref", DeleteBranchCmd{Name: "foo", DeleteRef: true}, false},
		{"blarn", nil, true},
	} {
		t.Run(tt.Input, func(t *testing.T) {
			cmd, err := ParseCmd(tt.Input)

			if tt.Err {
				if err == nil {
					t.Errorf("got err %v, want %v", err, tt.Err)
				}
				return
			} else if err != nil {
				t.Errorf("got unexpected err %v", err)
				return
			}

			if !reflect.DeepEqual(cmd, tt.Cmd) {
				t.Errorf("got %#v, want %#v", cmd, &tt.Cmd)
			}
		})
	}
}
