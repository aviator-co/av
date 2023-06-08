package sliceutils

// Subtract returns a slice containing all elements of a that are not in b.
func Subtract[T comparable](a, b []T) []T {
	var result []T
	for _, v := range a {
		if !Contains(b, v) {
			result = append(result, v)
		}
	}
	return result
}
