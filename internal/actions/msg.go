package actions

import (
	"fmt"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/colors"
	"github.com/kr/text"
	"os"
	"strings"
)

func msgRebaseResult(rebase *git.RebaseResult) {
	switch rebase.Status {
	case git.RebaseAlreadyUpToDate:
		_, _ = fmt.Fprint(os.Stderr, "  - already up to date\n")
	case git.RebaseUpdated:
		_, _ = fmt.Fprint(os.Stderr, "  - ", colors.Success("rebased without conflicts"), "\n")
	case git.RebaseConflict:
		_, _ = fmt.Fprint(os.Stderr,
			"  - ", colors.Failure("rebase conflict: ", rebase.ErrorHeadline), "\n",
			colors.Faint(text.Indent(strings.TrimSpace(rebase.Hint), "        ")),
			"\n",
		)
		_, _ = fmt.Fprint(os.Stderr,
			"  - resolve the conflicts and continue the sync with ", colors.CliCmd("av stack sync --continue"),
			"\n",
		)
		_, _ = fmt.Fprint(os.Stderr,
			"      - ",
			colors.Warning("NOTE: do not use the "), colors.CliCmd("git rebase"),
			colors.Warning(" command directly: use "), colors.CliCmd("av stack sync"),
			colors.Warning(" instead"), "\n",
		)
	}
}
