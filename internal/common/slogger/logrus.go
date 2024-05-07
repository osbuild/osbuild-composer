package slogger

import (
	"context"
	"github.com/osbuild/osbuild-composer/pkg/jobqueue"
	"github.com/sirupsen/logrus"
	"runtime"
)

type simpleLogrus struct {
	logger *logrus.Logger
}

func NewLogrusLogger(logger *logrus.Logger) jobqueue.SimpleLogger {
	newLogger := &simpleLogrus{logger: logger}
	logger.AddHook(&ctxHook{})
	return newLogger
}

type ctxKey int

const (
	LoggingFrameLogrusCtx ctxKey = iota
	LoggingFrameCtx       ctxKey = iota
)

type ctxHook struct {
}

func (h *ctxHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.DebugLevel,
		logrus.InfoLevel,
		logrus.WarnLevel,
		logrus.ErrorLevel,
		logrus.FatalLevel,
		logrus.PanicLevel,
	}
}

func (h *ctxHook) Fire(e *logrus.Entry) error {
	if e.Context != nil {
		if e.Context.Value(LoggingFrameLogrusCtx) != nil {
			frame := e.Context.Value(LoggingFrameLogrusCtx).(runtime.Frame)
			e.Caller = &frame
		}
	}

	return nil
}

func (l *simpleLogrus) logWithCaller() *logrus.Entry {
	// this function is necessary as logrus would report
	// the location of the wrapper function by default
	ctx := context.Background()
	rpc := make([]uintptr, 1)
	// logWithCaller is always 4 frames below the calling context
	n := runtime.Callers(4, rpc[:])
	if n < 1 {
		return l.logger.WithContext(ctx)
	}
	frame, _ := runtime.CallersFrames(rpc).Next()
	frameOverride := context.WithValue(ctx, LoggingFrameLogrusCtx, frame)
	return l.logger.WithContext(frameOverride)
}

func (s *simpleLogrus) log(level logrus.Level, err error, msg string, args ...string) {
	if len(args)%2 != 0 {
		panic("log arguments must be even (key value pairs)")
	}
	var fields = make(logrus.Fields, len(args)/2+1)
	for i := 0; i < len(args); i += 2 {
		k := args[i]
		v := args[i+1]
		fields[k] = v
	}
	if err != nil {
		fields["error"] = err.Error()
	}
	s.logWithCaller().WithFields(fields).Log(level, msg)
}

func (s *simpleLogrus) Info(msg string, args ...string) {
	s.log(logrus.InfoLevel, nil, msg, args...)
}

func (s *simpleLogrus) Error(err error, msg string, args ...string) {
	s.log(logrus.ErrorLevel, err, msg, args...)
}
