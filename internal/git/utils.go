package git

func ShortSha(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
