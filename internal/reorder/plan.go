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
	var cmds []Cmd

	branchNames := []string{rootBranch}
	branchNames = append(branchNames, meta.SubsequentBranches(tx, rootBranch)...)

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

		// Build the initial list of pick commands for this branch.
		var picks []PickCmd
		for _, object := range commitObjects {
			commit, err := git.ParseCommitContents(object.Contents)
			if err != nil {
				return nil, errors.WrapIff(err, "parsing commit %s", object.OID)
			}
			picks = append(picks, PickCmd{
				Commit:  git.ShortSha(object.OID),
				Comment: commit.MessageTitle(),
			})
		}

		// Reorder fixup!/squash! commits to sit immediately after their targets,
		// matching git's --autosquash behavior.
		picks = autosquashPickCmds(picks)

		cmds = append(cmds, branchCmd)
		for _, p := range picks {
			cmds = append(cmds, p)
		}
	}

	return cmds, nil
}

// autosquashPickCmds reorders picks so that fixup!/squash! commits are placed
// immediately after the most recent commit they target, matching git's
// --autosquash behavior. fixup! commits are set to PickModeFixup and squash!
// commits are set to PickModeSquash.
func autosquashPickCmds(picks []PickCmd) []PickCmd {
	type fixupInfo struct {
		mode        PickMode
		targetTitle string
	}

	fixups := make(map[int]fixupInfo)
	for i, p := range picks {
		if strings.HasPrefix(p.Comment, "fixup! ") {
			fixups[i] = fixupInfo{PickModeFixup, p.Comment[len("fixup! "):]}
		} else if strings.HasPrefix(p.Comment, "squash! ") {
			fixups[i] = fixupInfo{PickModeSquash, p.Comment[len("squash! "):]}
		}
	}

	if len(fixups) == 0 {
		return picks
	}

	// For each fixup, find the last non-fixup commit with a matching title.
	lastMatchIdx := make(map[int]int) // fixupIdx -> targetCommitIdx (-1 if not found)
	for fixIdx, info := range fixups {
		lastMatchIdx[fixIdx] = -1
		for i, p := range picks {
			if _, isFix := fixups[i]; !isFix && p.Comment == info.targetTitle {
				lastMatchIdx[fixIdx] = i
			}
		}
	}

	// Group fixup indices by the commit they should follow, preserving
	// their original relative order within each group.
	insertAfter := make(map[int][]int) // targetCommitIdx -> []fixupIdx
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

	// Build the result: emit each non-fixup commit followed by any fixups
	// that target it.
	var result []PickCmd
	for i, p := range picks {
		if _, isFix := fixups[i]; isFix {
			continue
		}
		result = append(result, p)
		for _, fixIdx := range insertAfter[i] {
			fixPick := picks[fixIdx]
			fixPick.Mode = fixups[fixIdx].mode
			result = append(result, fixPick)
		}
	}

	// Append any fixups whose target was not found in this branch,
	// annotated with a warning so the user can see the problem in the editor.
	for _, fixIdx := range unplaced {
		fixPick := picks[fixIdx]
		fixPick.Mode = fixups[fixIdx].mode
		fixPick.Comment = fmt.Sprintf(
			"WARNING: target commit %q not found in this branch",
			fixups[fixIdx].targetTitle,
		)
		result = append(result, fixPick)
	}

	return result
}
