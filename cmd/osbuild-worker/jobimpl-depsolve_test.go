package main_test

import (
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"

	"github.com/osbuild/images/pkg/dnfjson"

	worker "github.com/osbuild/osbuild-composer/cmd/osbuild-worker"
)

func makeMockEntry() (*logrus.Entry, *test.Hook) {
	logger, hook := test.NewNullLogger()
	return logger.WithField("test", "test"), hook
}

func TestWorkerClientErrorFromDnfJson(t *testing.T) {
	dnfJsonErr := dnfjson.Error{
		Kind:   "DepsolveError",
		Reason: "something is terribly wrong",
	}
	entry, hook := makeMockEntry()
	clientErr := worker.WorkerClientErrorFrom(dnfJsonErr, entry)
	assert.Equal(t, `Code: 20, Reason: DNF error occurred: DepsolveError, Details: something is terribly wrong`, clientErr.String())

	wrappedErr := fmt.Errorf("Wrap the error: %w", dnfJsonErr)
	clientErr = worker.WorkerClientErrorFrom(wrappedErr, entry)
	assert.Equal(t, `Code: 20, Reason: DNF error occurred: DepsolveError, Details: something is terribly wrong`, clientErr.String())

	assert.Equal(t, 0, len(hook.AllEntries()))
}

func TestWorkerClientErrorFromDnfJsonOtherKind(t *testing.T) {
	dnfJsonErr := dnfjson.Error{
		Kind:   "something-else",
		Reason: "something is terribly wrong",
	}
	entry, hook := makeMockEntry()
	clientErr := worker.WorkerClientErrorFrom(dnfJsonErr, entry)
	assert.Equal(t, `Code: 22, Reason: DNF error occurred: something-else, Details: something is terribly wrong`, clientErr.String())
	assert.Equal(t, 1, len(hook.AllEntries()))
	assert.Equal(t, "Unhandled dnf-json error in depsolve job: DNF error occurred: something-else: something is terribly wrong", hook.LastEntry().Message)
}

func TestWorkerClientErrorFromOtherError(t *testing.T) {
	otherErr := fmt.Errorf("some error")
	entry, hook := makeMockEntry()
	clientErr := worker.WorkerClientErrorFrom(otherErr, entry)
	assert.Equal(t, `Code: 23, Reason: rpmmd error in depsolve job, Details: some error`, clientErr.String())
	assert.Equal(t, 0, len(hook.AllEntries()))
}

func TestWorkerClientErrorFromNil(t *testing.T) {
	entry, hook := makeMockEntry()
	clientErr := worker.WorkerClientErrorFrom(nil, entry)
	assert.Equal(t, `Code: 23, Reason: rpmmd error in depsolve job, Details: <nil>`, clientErr.String())
	assert.Equal(t, 1, len(hook.AllEntries()))
	assert.Equal(t, "workerClientErrorFrom expected an error to be processed. Not nil", hook.LastEntry().Message)
}
