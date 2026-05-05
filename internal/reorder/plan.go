package reorder

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
)

// CreatePlan creates a reorder plan for the stack rooted at rootBranch.
func CreatePlan(
	ctx context.Context,
	repo *git.Repo,
	tx meta.ReadTx,
	rootBranch string,
) ([]Cmd, error) {
	branchNames := []string{rootBranch}
	branchNames = append(branchNames, meta.SubsequentBranches(tx, rootBranch)...)

	var cmds []Cmd
	for _, branchName := range branchNames {
		branch, _ := tx.Branch(branchName)

		branchCmd := StackBranchCmd{
			Name: branchName,
		}
		// Need to figure out the upstream commit to figure out the list of
		// commits associated with this branch.
		var upstreamCommit string
		// TODO: would be nice to show the user whether or not the branch is
		// 		already up-to-date with the parent.
		if branch.Parent.BranchingPointCommitHash != "" {
			branchCmd.Parent = branch.Parent.Name
			upstreamCommit = branch.Parent.BranchingPointCommitHash
		} else {
			trunkCommit, err := repo.MergeBase(ctx, branchName, "origin/"+branch.Parent.Name)
			if err != nil {
				return nil, err
			}
			branchCmd.Trunk = branch.Parent.Name + "@" + trunkCommit
			upstreamCommit = trunkCommit
		}

		// Figure out the commits that belong to this branch.
		// We'll use this to generate a "pick" command for each commit.
		commitIDs, err := repo.RevList(ctx, git.RevListOpts{
			Specifiers: []string{branchName, "^" + upstreamCommit},
			Reverse:    true,
		})
		if err != nil {
			return nil, err
		}

		// If no commits associated with this branch, bail out early and add a
		// helpful comment for the user.
		if len(commitIDs) == 0 {
			branchCmd.Comment = "this branch has no commits"
			cmds = append(cmds, branchCmd)
			continue
		}

		commitObjects, err := repo.GetRefs(ctx, &git.GetRefs{
			Revisions: commitIDs,
		})
		if err != nil {
			return nil, err
		}

		cmds = append(cmds, branchCmd)
		for _, object := range commitObjects {
			commit, err := git.ParseCommitContents(object.Contents)
			if err != nil {
				return nil, errors.WrapIff(err, "parsing commit %s", object.OID)
			}
			cmds = append(cmds, PickCmd{
				Commit:  object.OID,
				Comment: commit.MessageTitle(),
			})
		}
	}

	// Reorder fixup!/squash! commits to sit immediately after their targets
	// across the entire stack, matching git's --autosquash behavior.
	return autosquashCmds(cmds), nil
}

// autosquashCmds reorders fixup!/squash! picks so that each one is placed
// immediately after the most recent pick it targets anywhere in the stack,
// matching git's --autosquash behavior. StackBranchCmd entries are treated as
// opaque markers and preserved in place; a fixup targeting a commit in another
// branch is moved into that branch's section of the plan.
func autosquashCmds(cmds []Cmd) []Cmd {
	type fixupInfo struct {
		mode        PickMode
		targetTitle string
	}

	fixups := make(map[int]fixupInfo)
	for i, cmd := range cmds {
		p, ok := cmd.(PickCmd)
		if !ok {
			continue
		}
		if strings.HasPrefix(p.Comment, "fixup! ") {
			fixups[i] = fixupInfo{PickModeFixup, p.Comment[len("fixup! "):]}
		} else if strings.HasPrefix(p.Comment, "squash! ") {
			fixups[i] = fixupInfo{PickModeSquash, p.Comment[len("squash! "):]}
		}
	}

	if len(fixups) == 0 {
		return cmds
	}

	// For each fixup, find the last non-fixup pick with a matching title
	// anywhere in the stack.
	lastMatchIdx := make(map[int]int) // fixupIdx -> targetIdx (-1 if not found)
	for fixIdx, info := range fixups {
		lastMatchIdx[fixIdx] = -1
		for i, cmd := range cmds {
			p, ok := cmd.(PickCmd)
			if !ok {
				continue
			}
			if _, isFix := fixups[i]; !isFix && p.Comment == info.targetTitle {
				lastMatchIdx[fixIdx] = i
			}
		}
	}

	// Group fixup indices by the commit they should follow, preserving
	// their original relative order within each group.
	insertAfter := make(map[int][]int) // targetIdx -> []fixupIdx
	var unplaced []int
	for fixIdx, targetIdx := range lastMatchIdx {
		if targetIdx >= 0 {
			insertAfter[targetIdx] = append(insertAfter[targetIdx], fixIdx)
		} else {
			unplaced = append(unplaced, fixIdx)
		}
	}
	for targetIdx := range insertAfter {
		slices.Sort(insertAfter[targetIdx])
	}
	slices.Sort(unplaced)

	// Build the result: emit each non-fixup entry followed by any fixups
	// that target it.
	var result []Cmd
	for i, cmd := range cmds {
		if _, isFix := fixups[i]; isFix {
			continue // will be re-inserted after its target
		}
		result = append(result, cmd)
		for _, fixIdx := range insertAfter[i] {
			fixPick := cmds[fixIdx].(PickCmd)
			fixPick.Mode = fixups[fixIdx].mode
			result = append(result, fixPick)
		}
	}

	// Append fixups whose target was not found anywhere in the stack,
	// annotated with a warning so the user can see the problem in the editor.
	for _, fixIdx := range unplaced {
		fixPick := cmds[fixIdx].(PickCmd)
		fixPick.Mode = PickModePick
		fixPick.Comment = fmt.Sprintf(
			"WARNING: target commit %q not found in the stack",
			fixups[fixIdx].targetTitle,
		)
		result = append(result, fixPick)
	}

	return result
}
