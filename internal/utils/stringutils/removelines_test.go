package stringutils_test

import (
	"testing"

	"github.com/aviator-co/av/internal/utils/stringutils"
	"github.com/stretchr/testify/require"
)

func TestRemoveLines(t *testing.T) {
	input := `Auto-merging eggs
CONFLICT (content): Merge conflict in eggs
error: could not apply c29d5fb... spam
hint: Resolve all conflicts manually, mark them as resolved with
hint: "git add/rm <conflicted_files>", then run "git rebase --continue".
hint: You can instead skip this commit: run "git rebase --skip".
hint: To abort and get back to the state before "git rebase", run "git rebase --abort".
Could not apply c29d5fb... spam
`
	expected := `Auto-merging eggs
CONFLICT (content): Merge conflict in eggs
error: could not apply c29d5fb... spam
Could not apply c29d5fb... spam
`
	require.Equal(t, expected, stringutils.RemoveLines(input, "hint: "))
}
