package errutils

import "emperror.dev/errors"

// As is a wrapper around errors.As using generics that returns the concrete
// error type if err is of type T.
func As[T error](err error) (T, bool) {
	var concreteErr T
	if err == nil {
		return concreteErr, false
	}
	if errors.As(err, &concreteErr) {
		return concreteErr, true
	} else {
		return concreteErr, false
	}
}
