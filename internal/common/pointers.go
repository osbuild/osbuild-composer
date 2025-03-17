package common

func ToPtr[T any](x T) *T {
	return &x
}

// DerefOrDefault returns the dereferenced value of the given pointer or the
// default value for the type if unset.
func DerefOrDefault[T any](p *T) T {
	var v T
	if p != nil {
		v = *p
	}
	return v
}
