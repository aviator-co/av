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
