package logger

import (
	"github.com/sirupsen/logrus"
)

type EnvironmentHook struct {
	Channel string
}

func (h *EnvironmentHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.DebugLevel,
		logrus.InfoLevel,
		logrus.WarnLevel,
		logrus.ErrorLevel,
		logrus.FatalLevel,
		logrus.PanicLevel,
	}
}

func (h *EnvironmentHook) Fire(e *logrus.Entry) error {
	e.Data["channel"] = h.Channel

	return nil
}
