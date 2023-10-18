package logger

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

type SplunkHook struct {
	sl *SplunkLogger
}

func NewSplunkHook(host, port, token, source string) (*SplunkHook, error) {
	url := fmt.Sprintf("https://%s:%s/services/collector/event", host, port)
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	return &SplunkHook{
		sl: NewSplunkLogger(url, token, source, hostname),
	}, nil
}

func (sh *SplunkHook) Fire(entry *logrus.Entry) error {
	msg, err := entry.String()
	if err != nil {
		return err
	}

	return sh.sl.LogWithTime(entry.Time, msg)
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
