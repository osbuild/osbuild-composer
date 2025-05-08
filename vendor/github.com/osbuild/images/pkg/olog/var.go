package olog

import (
	"io"
	"log"
	"sync/atomic"
)

var logger atomic.Pointer[log.Logger]

func init() {
	SetDefault(nil)
}

// SetDefault sets the default logger to the provided logger. When nil is passed,
// the default logger is set to a no-op logger that discards all log messages.
func SetDefault(l *log.Logger) {
	if l == nil {
		logger.Store(log.New(io.Discard, "", 0))
		return
	}

	logger.Store(l)
}

// Default returns the default logger. If no logger has been set, it returns a
// no-op logger that discards all log messages.
func Default() *log.Logger {
	return logger.Load()
}
