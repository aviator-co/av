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
		{"squash", PickCmd{}, true},
		{"squash foo", PickCmd{Commit: "foo", Mode: PickModeSquash}, false},
		{"squash foo bar", PickCmd{}, true},
		{"s foo", PickCmd{Commit: "foo", Mode: PickModeSquash}, false},
		{"fixup", PickCmd{}, true},
		{"fixup foo", PickCmd{Commit: "foo", Mode: PickModeFixup}, false},
		{"fixup foo bar", PickCmd{}, true},
		{"f foo", PickCmd{Commit: "foo", Mode: PickModeFixup}, false},
		{"delete-branch", DeleteBranchCmd{}, true},
		{"delete-branch foo", DeleteBranchCmd{Name: "foo"}, false},
		{"delete-branch foo bar", DeleteBranchCmd{}, true},
		{"db foo --delete-git-ref", DeleteBranchCmd{Name: "foo", DeleteGitRef: true}, false},
		{"blarn", nil, true},
	} {
		t.Run(tt.Input, func(t *testing.T) {
			cmd, err := ParseCmd(tt.Input, nil)

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
				t.Errorf("got %#v, want %#v", cmd, tt.Cmd)
			}
		})
	}
}

func TestParseCmdResolvesShortHashes(t *testing.T) {
	shortToFull := map[string]string{
		"aaaaaaa": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"bbbbbbb": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}

	for _, tt := range []struct {
		input string
		cmd   Cmd
	}{
		{
			input: "pick bbbbbbb",
			cmd:   PickCmd{Commit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		},
		{
			input: "squash bbbbbbb",
			cmd:   PickCmd{Commit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Mode: PickModeSquash},
		},
		{
			input: "fixup bbbbbbb",
			cmd:   PickCmd{Commit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Mode: PickModeFixup},
		},
		{
			input: "stack-branch one --trunk main@aaaaaaa",
			cmd: StackBranchCmd{
				Name:  "one",
				Trunk: "main@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
		},
	} {
		t.Run(tt.input, func(t *testing.T) {
			cmd, err := ParseCmd(tt.input, shortToFull)
			if err != nil {
				t.Fatalf("got unexpected err %v", err)
			}
			if !reflect.DeepEqual(cmd, tt.cmd) {
				t.Errorf("got %#v, want %#v", cmd, tt.cmd)
			}
		})
	}
}
