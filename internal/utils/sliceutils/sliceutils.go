package sliceutils

// DeleteElement deletes the first element in the slice that equals the given
// value.
func DeleteElement[T comparable](slice []T, element T) []T {
	for i := range slice {
		if slice[i] == element {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// Replace replaces every instance of oldVal with newVal in the given slice.
// It returns true if
func Replace[T comparable](slice []T, oldVal T, newVal T) int {
	replaced := 0
	for i := range slice {
		if slice[i] == oldVal {
			slice[i] = newVal
			replaced++
		}
	}
	return replaced
}
