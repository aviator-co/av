package sliceutils

func Contains[T comparable](s []T, v T) bool {
	for _, e := range s {
		if e == v {
			return true
		}
	}
	return false
}
