package sliceutils

func AppendIfNotContains[T comparable](s []T, v T) []T {
	if Contains(s, v) {
		return s
	}
	return append(s, v)
}
