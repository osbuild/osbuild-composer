package slogger

import (
	"github.com/osbuild/osbuild-composer/pkg/jobqueue"
	"github.com/sirupsen/logrus"
)

type simpleLogrus struct {
	logger *logrus.Logger
}

func NewLogrusLogger(logger *logrus.Logger) jobqueue.SimpleLogger {
	return &simpleLogrus{logger: logger}
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
	s.logger.WithFields(fields).Log(level, msg)
}

func (s *simpleLogrus) Info(msg string, args ...string) {
	s.log(logrus.InfoLevel, nil, msg, args...)
}

func (s *simpleLogrus) Error(err error, msg string, args ...string) {
	s.log(logrus.ErrorLevel, err, msg, args...)
}
