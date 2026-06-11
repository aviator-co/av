package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseGitStatusLine_RenameWithSpaceInPath(t *testing.T) {
	// `git status --porcelain=v2` emits renamed entries as:
	//   2 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <Xscore> <path>\t<origPath>
	// The new path can legitimately contain spaces, so the parser must capture
	// everything up to the tab rather than splitting on the last space.
	line := "2 R. N... 100644 100644 100644 1111111 2222222 R100 new file.txt\told.txt"
	var st GitStatus
	parseGitStatusLine(line, &st)
	assert.Equal(t, []string{"new file.txt"}, st.StagedTrackedFiles)
	assert.Empty(t, st.UnstagedTrackedFiles)
}

func TestParseGitStatusLine_RenameNoSpace(t *testing.T) {
	line := "2 R. N... 100644 100644 100644 1111111 2222222 R100 newname.txt\toldname.txt"
	var st GitStatus
	parseGitStatusLine(line, &st)
	assert.Equal(t, []string{"newname.txt"}, st.StagedTrackedFiles)
}
