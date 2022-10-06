// Package slogger provide a simple logging interface with implementation for logrus.
package slogger

// SimpleLogger provides a structured logging methods for the jobqueue library.
type SimpleLogger interface {
	// Info creates an info-level message and arbitrary amount of key-value string pairs which
	// can be optionally mapped to fields by underlying implementations.
	Info(msg string, args ...string)

	// Error creates an error-level message and arbitrary amount of key-value string pairs which
	// can be optionally mapped to fields by underlying implementations. The first error argument
	// can be set to nil when no context error is available.
	Error(err error, msg string, args ...string)
}
