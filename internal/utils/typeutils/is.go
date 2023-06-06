package typeutils

func Is[T any](elt any) bool {
	_, ok := elt.(T)
	return ok
}
