package common

import (
	"context"
	"encoding/json"
	"io"
	"runtime"

	lslog "github.com/labstack/gommon/log"
	"github.com/sirupsen/logrus"
)

// EchoLogrusLogger extend logrus.Logger
type EchoLogrusLogger struct {
	logger *logrus.Logger
	ctx    context.Context
	writer *io.PipeWriter
}

func NewEchoLogrusLogger(logger *logrus.Logger, ctx context.Context) *EchoLogrusLogger {
	return &EchoLogrusLogger{
		logger: logger,
		ctx:    ctx,
		writer: logger.WithContext(ctx).Writer(),
	}
}

var commonLogger *EchoLogrusLogger

func init() {
	commonLogger = &EchoLogrusLogger{
		logger: logrus.StandardLogger(),
		ctx:    context.Background(),
		writer: logrus.StandardLogger().WithContext(context.Background()).Writer(),
	}

	runtime.SetFinalizer(commonLogger, close)
}

func close(l *EchoLogrusLogger) {
	if l.writer != nil {
		l.writer.Close()
	}
}

func Logger() *EchoLogrusLogger {
	return commonLogger
}

func toEchoLevel(level logrus.Level) lslog.Lvl {
	switch level {
	case logrus.DebugLevel:
		return lslog.DEBUG
	case logrus.InfoLevel:
		return lslog.INFO
	case logrus.WarnLevel:
		return lslog.WARN
	case logrus.ErrorLevel:
		return lslog.ERROR
	}

	return lslog.OFF
}

func (l *EchoLogrusLogger) Close() {
	if l.writer != nil {
		l.writer.Close()
	}
}

func (l *EchoLogrusLogger) Output() io.Writer {
	return l.writer
}

func (l *EchoLogrusLogger) SetOutput(w io.Writer) {
	// disable operations that would change behavior of global logrus logger.
}

func (l *EchoLogrusLogger) Level() lslog.Lvl {
	return toEchoLevel(l.logger.Level)
}

func (l *EchoLogrusLogger) SetLevel(v lslog.Lvl) {
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
	l.logger.WithContext(l.ctx).Print(i...)
}

func (l *EchoLogrusLogger) Printf(format string, args ...interface{}) {
	l.logger.WithContext(l.ctx).Printf(format, args...)
}

func (l *EchoLogrusLogger) Printj(j lslog.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.logger.WithContext(l.ctx).Println(string(b))
}

func (l *EchoLogrusLogger) Debug(i ...interface{}) {
	l.logger.WithContext(l.ctx).Debug(i...)
}

func (l *EchoLogrusLogger) Debugf(format string, args ...interface{}) {
	l.logger.WithContext(l.ctx).Debugf(format, args...)
}

func (l *EchoLogrusLogger) Debugj(j lslog.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.logger.WithContext(l.ctx).Debugln(string(b))
}

func (l *EchoLogrusLogger) Info(i ...interface{}) {
	l.logger.WithContext(l.ctx).Info(i...)
}

func (l *EchoLogrusLogger) Infof(format string, args ...interface{}) {
	l.logger.WithContext(l.ctx).Infof(format, args...)
}

func (l *EchoLogrusLogger) Infoj(j lslog.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.logger.WithContext(l.ctx).Infoln(string(b))
}

func (l *EchoLogrusLogger) Warn(i ...interface{}) {
	l.logger.WithContext(l.ctx).Warn(i...)
}

func (l *EchoLogrusLogger) Warnf(format string, args ...interface{}) {
	l.logger.WithContext(l.ctx).Warnf(format, args...)
}

func (l *EchoLogrusLogger) Warnj(j lslog.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.logger.WithContext(l.ctx).Warnln(string(b))
}

func (l *EchoLogrusLogger) Error(i ...interface{}) {
	l.logger.WithContext(l.ctx).Error(i...)
}

func (l *EchoLogrusLogger) Errorf(format string, args ...interface{}) {
	l.logger.WithContext(l.ctx).Errorf(format, args...)
}

func (l *EchoLogrusLogger) Errorj(j lslog.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.logger.WithContext(l.ctx).Errorln(string(b))
}

func (l *EchoLogrusLogger) Fatal(i ...interface{}) {
	l.logger.WithContext(l.ctx).Fatal(i...)
}

func (l *EchoLogrusLogger) Fatalf(format string, args ...interface{}) {
	l.logger.WithContext(l.ctx).Fatalf(format, args...)
}

func (l *EchoLogrusLogger) Fatalj(j lslog.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.logger.WithContext(l.ctx).Fatalln(string(b))
}

func (l *EchoLogrusLogger) Panic(i ...interface{}) {
	l.logger.WithContext(l.ctx).Panic(i...)
}

func (l *EchoLogrusLogger) Panicf(format string, args ...interface{}) {
	l.logger.WithContext(l.ctx).Panicf(format, args...)
}

func (l *EchoLogrusLogger) Panicj(j lslog.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.logger.WithContext(l.ctx).Panicln(string(b))
}

// Write method is heavily used by the stdlib log package and called
// from the weldr API.
func (l *EchoLogrusLogger) Write(p []byte) (n int, err error) {
	return l.writer.Write(p)
}
