package common

import (
	"context"
	"encoding/json"
	"io"
	"runtime"

	"github.com/labstack/gommon/log"
	"github.com/sirupsen/logrus"
)

// EchoLogrusLogger extend logrus.Logger
type EchoLogrusLogger struct {
	*logrus.Logger
	Ctx context.Context
}

var commonLogger = &EchoLogrusLogger{
	Logger: logrus.StandardLogger(),
	Ctx: context.Background(),
}

type ctxKey int

const (
	LoggingFrameCtx ctxKey = iota
)

func Logger() *EchoLogrusLogger {
	return commonLogger
}

func toEchoLevel(level logrus.Level) log.Lvl {
	switch level {
	case logrus.DebugLevel:
		return log.DEBUG
	case logrus.InfoLevel:
		return log.INFO
	case logrus.WarnLevel:
		return log.WARN
	case logrus.ErrorLevel:
		return log.ERROR
	}

	return log.OFF
}

// add the context and caller to the fields
// as logrus will report "echo_logrus.go" otherwise
func (l *EchoLogrusLogger) fixCaller() *logrus.Entry {
	rpc := make([]uintptr, 1)
	// fixCaller is always 3 frames below the calling context
	n := runtime.Callers(3, rpc[:])
	if n < 1 {
		return l.Logger.WithContext(l.Ctx)
	}
	frame, _ := runtime.CallersFrames(rpc).Next()
	frameOverride := context.WithValue(l.Ctx, LoggingFrameCtx, frame)
	return l.Logger.WithContext(frameOverride)
}

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
		if e.Context.Value(LoggingFrameCtx) != nil {
			frame := e.Context.Value(LoggingFrameCtx).(runtime.Frame)
			e.Caller = &frame
		}
	}

	return nil
}

func init() {
	commonLogger.Logger.AddHook(&ctxHook{})
}

func (l *EchoLogrusLogger) Output() io.Writer {
	return l.Out
}

func (l *EchoLogrusLogger) SetOutput(w io.Writer) {
	// disable operations that would change behavior of global logrus logger.
}

func (l *EchoLogrusLogger) Level() log.Lvl {
	return toEchoLevel(l.Logger.Level)
}

func (l *EchoLogrusLogger) SetLevel(v log.Lvl) {
	// disable operations that would change behavior of global logrus logger.
}

func (l *EchoLogrusLogger) SetHeader(h string) {
}

func (l *EchoLogrusLogger) Prefix() string {
	return ""
}

func (l *EchoLogrusLogger) SetPrefix(p string) {
}

func (l *EchoLogrusLogger) Print(i ...interface{}) {
	l.fixCaller().Print(i...)
}

func (l *EchoLogrusLogger) Printf(format string, args ...interface{}) {
	l.fixCaller().Printf(format, args...)
}

func (l *EchoLogrusLogger) Printj(j log.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.fixCaller().Println(string(b))
}

func (l *EchoLogrusLogger) Debug(i ...interface{}) {
	l.fixCaller().Debug(i...)
}

func (l *EchoLogrusLogger) Debugf(format string, args ...interface{}) {
	l.fixCaller().Debugf(format, args...)
}

func (l *EchoLogrusLogger) Debugj(j log.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.fixCaller().Debugln(string(b))
}

func (l *EchoLogrusLogger) Info(i ...interface{}) {
	l.fixCaller().Info(i...)
}

func (l *EchoLogrusLogger) Infof(format string, args ...interface{}) {
	l.fixCaller().Infof(format, args...)
}

func (l *EchoLogrusLogger) Infoj(j log.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.fixCaller().Infoln(string(b))
}

func (l *EchoLogrusLogger) Warn(i ...interface{}) {
	l.fixCaller().Warn(i...)
}

func (l *EchoLogrusLogger) Warnf(format string, args ...interface{}) {
	l.fixCaller().Warnf(format, args...)
}

func (l *EchoLogrusLogger) Warnj(j log.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.fixCaller().Warnln(string(b))
}

func (l *EchoLogrusLogger) Error(i ...interface{}) {
	l.fixCaller().Error(i...)
}

func (l *EchoLogrusLogger) Errorf(format string, args ...interface{}) {
	l.fixCaller().Errorf(format, args...)
}

func (l *EchoLogrusLogger) Errorj(j log.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.fixCaller().Errorln(string(b))
}

func (l *EchoLogrusLogger) Fatal(i ...interface{}) {
	l.fixCaller().Fatal(i...)
}

func (l *EchoLogrusLogger) Fatalf(format string, args ...interface{}) {
	l.fixCaller().Fatalf(format, args...)
}

func (l *EchoLogrusLogger) Fatalj(j log.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.fixCaller().Fatalln(string(b))
}

func (l *EchoLogrusLogger) Panic(i ...interface{}) {
	l.fixCaller().Panic(i...)
}

func (l *EchoLogrusLogger) Panicf(format string, args ...interface{}) {
	l.fixCaller().Panicf(format, args...)
}

func (l *EchoLogrusLogger) Panicj(j log.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.fixCaller().Panicln(string(b))
}
