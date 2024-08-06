package git

import (
	"regexp"
	"strings"
)

// GitStatus is the status of the git repository.
//
// This uses the same format as the `git status --porcelain=v2` command. See
// https://git-scm.com/docs/git-status#_porcelain_format_version_2 for the details.
type GitStatus struct {
	// OID is the object ID of the commit.
	//
	// This can be an empty string. If it is empty, it means that the repository is just created
	// and there's no commit at all.
	OID string

	// CurrentBranch is the name of the current branch, without 'refs/heads/'.
	//
	// This can be an empty string. If it is empty, it means that the repository is at the
	// detached state.
	CurrentBranch string

	// UnstagedTrackedFiles is the list of the paths of the unstaged tracked files.
	UnstagedTrackedFiles []string
	// StageTrackedFiles is the list of the paths of the staged tracked files.
	StagedTrackedFiles []string
	// UnmergedFiles is the list of the paths of the unmerged files.
	UnmergedFiles []string
	// UntrackedFiles is the list of the paths of the untracked files.
	UntrackedFiles []string
}

func (st GitStatus) IsCleanIgnoringUntracked() bool {
	return len(st.UnstagedTrackedFiles) == 0 && len(st.StagedTrackedFiles) == 0 &&
		len(st.UnmergedFiles) == 0
}

func (st GitStatus) IsClean() bool {
	return st.IsCleanIgnoringUntracked() && len(st.UntrackedFiles) == 0
}

var (
	patternBranchOID        = regexp.MustCompile(`# branch\.oid ([0-9a-f]+)`)
	patternBranchOIDInitial = regexp.MustCompile(`# branch\.oid \(initial\)`)
	patternBranchHead       = regexp.MustCompile(`# branch\.head (.+)`)
	patternFile1            = regexp.MustCompile(
		`1 (..) .... ...... ...... ...... [0-9a-f]+ [0-9a-f]+ (.+)`,
	)
	patternFile2 = regexp.MustCompile(
		`2 (..) .... ...... ...... ...... [0-9a-f]+ [0-9a-f]+ .+ (.+)\t.+`,
	)
	patternFileUnmerged = regexp.MustCompile(
		`u .. .... ...... ...... ...... .... [0-9a-f]+ [0-9a-f]+ [0-9a-f]+ (.+)`,
	)
	patternFileUntracked = regexp.MustCompile(`\? (.+)`)
)

func (r *Repo) Status() (GitStatus, error) {
	body, err := r.Git("status", "--porcelain=v2", "--branch", "--untracked-files")
	if err != nil {
		return GitStatus{}, err
	}
	st := GitStatus{}
	for _, line := range strings.Split(body, "\n") {
		parseGitStatusLine(line, &st)
	}
	return st, nil
}

func parseGitStatusLine(line string, st *GitStatus) {
	if matches := patternBranchOID.FindStringSubmatch(line); len(matches) > 0 {
		st.OID = matches[1]
		return
	}
	if patternBranchOIDInitial.MatchString(line) {
		st.OID = ""
		return
	}
	if matches := patternBranchHead.FindStringSubmatch(line); len(matches) > 0 {
		if matches[1] == "(detached)" {
			st.CurrentBranch = ""
		} else {
			st.CurrentBranch = matches[1]
		}
		return
	}
	if matches := patternFile1.FindStringSubmatch(line); len(matches) > 0 {
		xy := matches[1]
		if xy[0] != '.' {
			st.StagedTrackedFiles = append(st.StagedTrackedFiles, matches[2])
		}
		if xy[1] != '.' {
			st.UnstagedTrackedFiles = append(st.UnstagedTrackedFiles, matches[2])
		}
		return
	}
	if matches := patternFile2.FindStringSubmatch(line); len(matches) > 0 {
		xy := matches[1]
		if xy[0] != '.' {
			st.StagedTrackedFiles = append(st.StagedTrackedFiles, matches[2])
		}
		if xy[1] != '.' {
			st.UnstagedTrackedFiles = append(st.UnstagedTrackedFiles, matches[2])
		}
		return
	}
	if matches := patternFileUnmerged.FindStringSubmatch(line); len(matches) > 0 {
		st.UnmergedFiles = append(st.UnmergedFiles, matches[1])
		return
	}
	if matches := patternFileUntracked.FindStringSubmatch(line); len(matches) > 0 {
		st.UntrackedFiles = append(st.UntrackedFiles, matches[1])
		return
	}
}
