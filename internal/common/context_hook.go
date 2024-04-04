package common

import (
	"github.com/sirupsen/logrus"
)

type ContextHook struct{}

func (h *ContextHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.DebugLevel,
		logrus.InfoLevel,
		logrus.WarnLevel,
		logrus.ErrorLevel,
		logrus.FatalLevel,
		logrus.PanicLevel,
	}
}

func (h *ContextHook) Fire(e *logrus.Entry) error {
	if e.Context == nil {
		return nil
	}

	if val := e.Context.Value(operationIDKeyCtx); val != nil {
		e.Data["operation_id"] = val
	}
	if val := e.Context.Value(externalIDKeyCtx); val != nil {
		e.Data["external_id"] = val
	}

	return nil
}
