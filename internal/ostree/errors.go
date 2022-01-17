package ostree

import "fmt"

// ResolveRefError is returned when there is a failure to resolve the
// reference.
type ResolveRefError struct {
	msg string
}

func (e ResolveRefError) Error() string {
	return e.msg
}

// NewResolveRefError creates and returns a new ResolveRefError with a given
// formatted message.
func NewResolveRefError(msg string, args ...interface{}) ResolveRefError {
	return ResolveRefError{msg: fmt.Sprintf(msg, args...)}
}

// InvalidParamsError is returned when a parameter is invalid (e.g., malformed
// or contains illegal characters).
type InvalidParameterError struct {
	msg string
}

func (e InvalidParameterError) Error() string {
	return e.msg
}

// NewInvalidParameterError creates and returns a new InvalidParameterError
// with a given formatted message.
func NewInvalidParameterError(msg string, args ...interface{}) InvalidParameterError {
	return InvalidParameterError{msg: fmt.Sprintf(msg, args...)}
}
