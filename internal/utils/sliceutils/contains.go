package sliceutils

import "slices"

func Contains[T comparable](s []T, v T) bool {
	return slices.Contains(s, v)
}
