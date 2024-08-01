package common

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func makeLogrus(buf *bytes.Buffer) *logrus.Logger {
	return &logrus.Logger{
		Out: buf,
		Formatter: &logrus.TextFormatter{
			DisableTimestamp: true,
			DisableColors:    true,
		},
		Hooks: make(logrus.LevelHooks),
		Level: logrus.DebugLevel,
	}

}

func TestInfoWithEnvironment(t *testing.T) {
	buf := &bytes.Buffer{}
	l := makeLogrus(buf)
	l.AddHook(&EnvironmentHook{Channel: "test_framework"})
	l.Info("test message")
	require.Equal(t, "level=info msg=\"test message\" channel=test_framework\n", buf.String())
}
