package slogger

import (
	"bytes"
	"errors"
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

func TestInfo(t *testing.T) {
	buf := &bytes.Buffer{}
	l := makeLogrus(buf)
	sl := NewLogrusLogger(l)
	sl.Info("test")
	require.Equal(t, "level=info msg=test\n", buf.String())
}

func TestError(t *testing.T) {
	buf := &bytes.Buffer{}
	l := makeLogrus(buf)
	sl := NewLogrusLogger(l)
	sl.Error(errors.New("e"), "test")
	require.Equal(t, "level=error msg=test error=e\n", buf.String())
}

func TestErrorIsNil(t *testing.T) {
	buf := &bytes.Buffer{}
	l := makeLogrus(buf)
	sl := NewLogrusLogger(l)
	sl.Error(nil, "test")
	require.Equal(t, "level=error msg=test\n", buf.String())
}

func TestInfoWithFields(t *testing.T) {
	buf := &bytes.Buffer{}
	l := makeLogrus(buf)
	sl := NewLogrusLogger(l)
	sl.Info("test", "key", "value")
	require.Equal(t, "level=info msg=test key=value\n", buf.String())
}

func TestErrorWithFields(t *testing.T) {
	buf := &bytes.Buffer{}
	l := makeLogrus(buf)
	sl := NewLogrusLogger(l)
	sl.Error(errors.New("e"), "test", "key", "value")
	require.Equal(t, "level=error msg=test error=e key=value\n", buf.String())
}
