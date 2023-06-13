package reorder

import (
	"strings"

	"github.com/aviator-co/av/internal/utils/sliceutils"

	"github.com/aviator-co/av/internal/editor"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/typeutils"
)

// EditPlan opens the user's editor and allows them to edit the plan.
func EditPlan(repo *git.Repo, plan []Cmd) ([]Cmd, error) {
	text := strings.Builder{}
	for i, cmd := range plan {
		if i > 0 && typeutils.Is[StackBranchCmd](cmd) {
			// Write an extra newline at the start of each branch command
			// (other than the first) to create a visual separation between
			// branches.
			text.WriteString("\n")
		}
		text.WriteString(cmd.String())
		text.WriteString("\n")
	}

	res, err := editor.Launch(repo, editor.Config{
		Text:              text.String(),
		CommentPrefix:     "#",
		EndOfLineComments: true,
	})
	if err != nil {
		return nil, err
	}

	var newPlan []Cmd
	lines := strings.Split(res, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cmd, err := ParseCmd(line)
		if err != nil {
			return nil, err
		}
		newPlan = append(newPlan, cmd)
	}

	return newPlan, nil
}

type PlanDiff struct {
	RemovedBranches []string
	AddedBranches   []string
}

func Diff(old []Cmd, new []Cmd) PlanDiff {

	var oldBranches []string
	for _, cmd := range old {
		if sb, ok := cmd.(StackBranchCmd); ok {
			oldBranches = append(oldBranches, sb.Name)
		}
	}

	var newBranches []string
	for _, cmd := range new {
		if sb, ok := cmd.(StackBranchCmd); ok {
			newBranches = append(newBranches, sb.Name)
		}
	}

	return PlanDiff{
		RemovedBranches: sliceutils.Subtract(oldBranches, newBranches),
		AddedBranches:   sliceutils.Subtract(newBranches, oldBranches),
	}
}
