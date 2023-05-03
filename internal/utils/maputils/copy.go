package maputils

// Copy returns a (shallow) copy of the given map.
func Copy[K comparable, T any](m map[K]T) map[K]T {
	c := make(map[K]T, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}
