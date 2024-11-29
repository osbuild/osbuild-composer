package common

import (
	"github.com/sirupsen/logrus"
)

type BuildHook struct {
}

func (h *BuildHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.DebugLevel,
		logrus.InfoLevel,
		logrus.WarnLevel,
		logrus.ErrorLevel,
		logrus.FatalLevel,
		logrus.PanicLevel,
	}
}

func (h *BuildHook) Fire(e *logrus.Entry) error {
	e.Data["build_commit"] = BuildCommit
	e.Data["build_time"] = BuildTime

	return nil
}
