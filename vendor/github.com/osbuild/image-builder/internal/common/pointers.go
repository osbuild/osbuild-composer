package common

func ToPtr[T any](x T) *T {
	return &x
}

// ClonePtr returns a new pointer to the same value as the input pointer.
// It returns nil if the input pointer is nil. If the value is a complex type,
// the returned pointer will be a shallow copy of the input pointer.
func ClonePtr[T any](x *T) *T {
	if x == nil {
		return nil
	}
	return ToPtr(*x)
}

// ValueOrEmpty returns the value from a given pointer. If ref is nil,
// a zero value of type T will be returned.
func ValueOrEmpty[T any](ref *T) (value T) {
	if ref != nil {
		value = *ref
	}
	return
}
