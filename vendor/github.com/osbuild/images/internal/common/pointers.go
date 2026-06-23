package common

func ToPtr[T any](x T) *T {
	return &x
}

// ValueOrEmpty returns the value from a given pointer. If ref is nil,
// a zero value of type T will be returned.
func ValueOrEmpty[T any](ref *T) (value T) {
	if ref != nil {
		value = *ref
	}
	return
}
