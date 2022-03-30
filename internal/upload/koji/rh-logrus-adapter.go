package koji

import (
	"strings"

	"github.com/sirupsen/logrus"
)

type LeveledLogrus struct {
	*logrus.Logger
}

const monitoringKeyword = "retrying"

func fields(keysAndValues ...interface{}) map[string]interface{} {
	fields := make(map[string]interface{})

	for i := 0; i < len(keysAndValues)-1; i += 2 {
		fields[keysAndValues[i].(string)] = keysAndValues[i+1]
	}

	return fields
}

func (l *LeveledLogrus) Error(msg string, keysAndValues ...interface{}) {
	l.WithFields(fields(keysAndValues...)).Error(msg)
}

func (l *LeveledLogrus) Info(msg string, keysAndValues ...interface{}) {
	l.WithFields(fields(keysAndValues...)).Info(msg)
}
func (l *LeveledLogrus) Debug(msg string, keysAndValues ...interface{}) {
	if strings.Contains(msg, monitoringKeyword) {
		l.WithFields(fields(keysAndValues...)).Info(msg)
	} else {
		l.WithFields(fields(keysAndValues...)).Debug(msg)
	}
}

func (l *LeveledLogrus) Warn(msg string, keysAndValues ...interface{}) {
	l.WithFields(fields(keysAndValues...)).Warn(msg)
}
