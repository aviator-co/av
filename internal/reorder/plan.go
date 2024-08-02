package reorder

import (
	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
	"github.com/aviator-co/av/internal/meta"
)

// CreatePlan creates a reorder plan for the stack rooted at rootBranch.
func CreatePlan(repo *git.Repo, tx meta.ReadTx, rootBranch string) ([]Cmd, error) {
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
		if branch.Parent.Head != "" {
			branchCmd.Parent = branch.Parent.Name
			upstreamCommit = branch.Parent.Head
		} else {
			trunkCommit, err := repo.MergeBase(branchName, "origin/"+branch.Parent.Name)
			if err != nil {
				return nil, err
			}
			branchCmd.Trunk = branch.Parent.Name + "@" + trunkCommit
			upstreamCommit = trunkCommit
		}

		// Figure out the commits that belong to this branch.
		// We'll use this to generate a "pick" command for each commit.
		commitIDs, err := repo.RevList(git.RevListOpts{
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

		commitObjects, err := repo.GetRefs(&git.GetRefs{
			Revisions: commitIDs,
		})
		if err != nil {
			return nil, err
		}

		// Append the "stack-branch" command and each "pick" command to the plan
		cmds = append(cmds, branchCmd)
		for _, object := range commitObjects {
			commit, err := git.ParseCommitContents(object.Contents)
			if err != nil {
				return nil, errors.WrapIff(err, "parsing commit %s", object.OID)
			}
			cmds = append(cmds, PickCmd{
				Commit:  git.ShortSha(object.OID),
				Comment: commit.MessageTitle(),
			})
		}
	}

	return cmds, nil
}
