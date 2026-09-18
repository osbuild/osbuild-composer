package logger

import (
	"context"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

type SplunkHook struct {
	sl *SplunkLogger
}

func NewSplunkHook(context context.Context, host, port, token, source string) (*SplunkHook, error) {
	url := fmt.Sprintf("https://%s:%s/services/collector/event", host, port)
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	return &SplunkHook{
		sl: NewSplunkLogger(context, url, token, source, hostname),
	}, nil
}

func (sh *SplunkHook) Fire(entry *logrus.Entry) error {
	msg, err := entry.String()
	if err != nil {
		return err
	}

	event := SplunkEvent{
		Message:   msg,
		ComposeID: stringField(entry.Data, "compose_id"),
		JobID:     stringField(entry.Data, "job_id"),
	}
	return sh.sl.LogEventWithTime(entry.Time, event)
}

func stringField(data logrus.Fields, key string) string {
	v, ok := data[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func (sh *SplunkHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}
