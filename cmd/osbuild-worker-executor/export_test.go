package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	logrusTest "github.com/sirupsen/logrus/hooks/test"
)

var (
	Run = run

	HandleIncludedSources = handleIncludedSources
)

func MockLogger() (hook *logrusTest.Hook, restore func()) {
	saved := logrusNew
	logger, hook := logrusTest.NewNullLogger()
	logrusNew = func() *logrus.Logger {
		return logger
	}
	logger.SetLevel(logrus.DebugLevel)

	return hook, func() {
		logrusNew = saved
	}
}

func MockOsbuildBinary(t *testing.T, new string) (restore func()) {
	t.Helper()

	saved := osbuildBinary

	tmpdir := t.TempDir()
	osbuildBinary = filepath.Join(tmpdir, "fake-osbuild")
	/* #nosec G306 */
	if err := os.WriteFile(osbuildBinary, []byte(new), 0755); err != nil {
		t.Fatal(err)
	}

	return func() {
		osbuildBinary = saved
	}
}
