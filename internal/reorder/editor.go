package reorder

import (
	"context"
	"strings"

	"github.com/aviator-co/av/internal/editor"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/utils/sliceutils"
	"github.com/aviator-co/av/internal/utils/typeutils"
)

// EditPlan opens the user's editor and allows them to edit the plan.
func EditPlan(ctx context.Context, repo *git.Repo, plan []Cmd) ([]Cmd, error) {
	shortToFull := make(map[string]string)
	text := strings.Builder{}
	for i, cmd := range plan {
		if i > 0 && typeutils.Is[StackBranchCmd](cmd) {
			// Write an extra newline at the start of each branch command
			// (other than the first) to create a visual separation between
			// branches.
			text.WriteString("\n")
		}
		text.WriteString(cmd.EditorString(shortToFull))
		text.WriteString("\n")
	}
	text.WriteString(instructionsText)

	res, err := editor.Launch(ctx, repo, editor.Config{
		Text:              text.String(),
		CommentPrefix:     "#",
		EndOfLineComments: true,
	})
	if err != nil {
		return nil, err
	}

	var newPlan []Cmd
	lines := strings.SplitSeq(res, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cmd, err := ParseCmd(line, shortToFull)
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

func Diff(oldCmds []Cmd, newCmds []Cmd) PlanDiff {
	var oldBranches []string
	for _, cmd := range oldCmds {
		if sb, ok := cmd.(StackBranchCmd); ok {
			oldBranches = append(oldBranches, sb.Name)
		}
	}

	var newBranches []string
	for _, cmd := range newCmds {
		if sb, ok := cmd.(StackBranchCmd); ok {
			newBranches = append(newBranches, sb.Name)
		}
	}

	return PlanDiff{
		RemovedBranches: sliceutils.Subtract(oldBranches, newBranches),
		AddedBranches:   sliceutils.Subtract(newBranches, oldBranches),
	}
}

const instructionsText = `
# Instructions:
#
# Commands:
# sb, stack-branch <branch-name> [--parent <parent-branch-name> | --trunk <trunk-branch-name>]
#         Create a new branch as part of a stack. If parent is not specified,
#         the previous branch in the stack is used (if any). If trunk is
#         specified, the branch is rooted from the given branch.
#         trunk-branch-name can be either a branch name or a branch name with a
#         commit ID in the format "<branch-name>@<commit-id>".
# p, pick <commit-id>
#         Pick a commit to be included in the stack. Only valid after a
#         stack-branch command.
# s, squash <commit-id>
#         Like pick, but squash the commit into the previous commit. The editor
#         will open to combine the two commit messages.
# f, fixup <commit-id>
#         Like squash, but discard the commit message and keep only the previous
#         commit's message.
#
# Commits with a "fixup!" or "squash!" message prefix (created by
# "git commit --fixup" or "git commit --squash") are automatically placed
# after their target commit and set to the appropriate action.
`
