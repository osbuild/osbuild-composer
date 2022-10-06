package noop

import (
	"github.com/osbuild/osbuild-composer/pkg/slogger"
)

type noopLogger struct {
}

func NewNoopLogger() slogger.SimpleLogger {
	return &noopLogger{}
}

func (s *noopLogger) Info(_ string, _ ...string) {
}

func (s *noopLogger) Error(_ error, _ string, _ ...string) {
}
