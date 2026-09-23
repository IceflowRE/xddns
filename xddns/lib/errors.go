package lib

type errorUnwraper interface {
	Unwrap() []error
}

// UnwrapErrors returns the underlying errors of the given error if it implements the Unwrap() []error method.
func UnwrapErrors(err error) []error {
	if uErr, ok := err.(errorUnwraper); ok {
		return uErr.Unwrap()
	}

	return nil
}
