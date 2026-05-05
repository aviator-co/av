package reorder

import "github.com/aviator-co/av/internal/git"

func shortCommitHash(commit string) string {
	if !isFullCommitHash(commit) {
		return commit
	}
	return git.ShortSha(commit)
}

func isFullCommitHash(commit string) bool {
	if len(commit) != 40 {
		return false
	}
	for _, r := range commit {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}
