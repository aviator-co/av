package textutils

func Pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
