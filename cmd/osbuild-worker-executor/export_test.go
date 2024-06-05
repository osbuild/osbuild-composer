package main

import (
	"io/ioutil"
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
	if err := ioutil.WriteFile(osbuildBinary, []byte(new), 0755); err != nil {
		t.Fatal(err)
	}

	return func() {
		osbuildBinary = saved
	}
}
