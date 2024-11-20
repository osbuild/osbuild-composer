package main_test

import (
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	logrusTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"

	main "github.com/osbuild/osbuild-composer/cmd/osbuild-worker"
)

func TestCatchesPanic(t *testing.T) {
	restore := main.MockRun(func() {
		// simulate a crash in the main code
		var foo *int
		println(*foo)
	})
	defer restore()

	// logrus setup is a bit cumbersome as we need to modify both
	// the standard logger and add a mock logger.
	var exitCalls []int
	logrus.StandardLogger().ExitFunc = func(exitCode int) {
		exitCalls = append(exitCalls, exitCode)
	}
	logrus.SetOutput(io.Discard)
	_, hook := logrusTest.NewNullLogger()
	logrus.AddHook(hook)

	main.Main()
	// ensure both message and stracktrace are reported in full
	assert.Equal(t, logrus.FatalLevel, hook.LastEntry().Level)
	msg := hook.LastEntry().Message
	assert.Contains(t, msg, "worker crashed: runtime error: invalid memory address or nil pointer dereference")
	assert.Contains(t, msg, "runtime/debug.Stack()")
	assert.Contains(t, msg, "osbuild-worker_test.TestCatchesPanic.func1()")

	assert.Equal(t, []int{1}, exitCalls)
}
