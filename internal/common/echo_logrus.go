package common

import (
	"context"
	"encoding/json"
	"io"

	lslog "github.com/labstack/gommon/log"
	"github.com/sirupsen/logrus"
)

// EchoLogrusLogger extend logrus.Logger
type EchoLogrusLogger struct {
	*logrus.Logger
	Ctx context.Context
}

var commonLogger = &EchoLogrusLogger{
	Logger: logrus.StandardLogger(),
	Ctx:    context.Background(),
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

func (l *EchoLogrusLogger) Output() io.Writer {
	return l.Out
}

func (l *EchoLogrusLogger) SetOutput(w io.Writer) {
	// disable operations that would change behavior of global logrus logger.
}

func (l *EchoLogrusLogger) Level() lslog.Lvl {
	return toEchoLevel(l.Logger.Level)
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
	l.Logger.WithContext(l.Ctx).Print(i...)
}

func (l *EchoLogrusLogger) Printf(format string, args ...interface{}) {
	l.Logger.WithContext(l.Ctx).Printf(format, args...)
}

func (l *EchoLogrusLogger) Printj(j lslog.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.Logger.WithContext(l.Ctx).Println(string(b))
}

func (l *EchoLogrusLogger) Debug(i ...interface{}) {
	l.Logger.WithContext(l.Ctx).Debug(i...)
}

func (l *EchoLogrusLogger) Debugf(format string, args ...interface{}) {
	l.Logger.WithContext(l.Ctx).Debugf(format, args...)
}

func (l *EchoLogrusLogger) Debugj(j lslog.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.Logger.WithContext(l.Ctx).Debugln(string(b))
}

func (l *EchoLogrusLogger) Info(i ...interface{}) {
	l.Logger.WithContext(l.Ctx).Info(i...)
}

func (l *EchoLogrusLogger) Infof(format string, args ...interface{}) {
	l.Logger.WithContext(l.Ctx).Infof(format, args...)
}

func (l *EchoLogrusLogger) Infoj(j lslog.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.Logger.WithContext(l.Ctx).Infoln(string(b))
}

func (l *EchoLogrusLogger) Warn(i ...interface{}) {
	l.Logger.WithContext(l.Ctx).Warn(i...)
}

func (l *EchoLogrusLogger) Warnf(format string, args ...interface{}) {
	l.Logger.WithContext(l.Ctx).Warnf(format, args...)
}

func (l *EchoLogrusLogger) Warnj(j lslog.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.Logger.WithContext(l.Ctx).Warnln(string(b))
}

func (l *EchoLogrusLogger) Error(i ...interface{}) {
	l.Logger.WithContext(l.Ctx).Error(i...)
}

func (l *EchoLogrusLogger) Errorf(format string, args ...interface{}) {
	l.Logger.WithContext(l.Ctx).Errorf(format, args...)
}

func (l *EchoLogrusLogger) Errorj(j lslog.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.Logger.WithContext(l.Ctx).Errorln(string(b))
}

func (l *EchoLogrusLogger) Fatal(i ...interface{}) {
	l.Logger.WithContext(l.Ctx).Fatal(i...)
}

func (l *EchoLogrusLogger) Fatalf(format string, args ...interface{}) {
	l.Logger.WithContext(l.Ctx).Fatalf(format, args...)
}

func (l *EchoLogrusLogger) Fatalj(j lslog.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.Logger.WithContext(l.Ctx).Fatalln(string(b))
}

func (l *EchoLogrusLogger) Panic(i ...interface{}) {
	l.Logger.WithContext(l.Ctx).Panic(i...)
}

func (l *EchoLogrusLogger) Panicf(format string, args ...interface{}) {
	l.Logger.WithContext(l.Ctx).Panicf(format, args...)
}

func (l *EchoLogrusLogger) Panicj(j lslog.JSON) {
	b, err := json.Marshal(j)
	if err != nil {
		panic(err)
	}
	l.Logger.WithContext(l.Ctx).Panicln(string(b))
}

func (l *EchoLogrusLogger) Write(p []byte) (n int, err error) {
	// Writer() from logrus returns PIPE that needs to be closed
	w := l.Logger.WithContext(l.Ctx).Writer()
	defer w.Close()

	return w.Write(p)
}
