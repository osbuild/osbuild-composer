package main_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/images/pkg/dnfjson"

	worker "github.com/osbuild/osbuild-composer/cmd/osbuild-worker"
)

func TestWorkerClientErrorFromDnfJson(t *testing.T) {
	dnfJsonErr := dnfjson.Error{
		Kind:   "DepsolveError",
		Reason: "something is terribly wrong",
	}
	clientErr, err := worker.WorkerClientErrorFrom(dnfJsonErr)
	assert.NoError(t, err)
	assert.Equal(t, `Code: 20, Reason: DNF error occurred: DepsolveError, Details: something is terribly wrong`, clientErr.String())
}

func TestWorkerClientErrorFromOtherError(t *testing.T) {
	otherErr := fmt.Errorf("some error")
	clientErr, err := worker.WorkerClientErrorFrom(otherErr)
	// XXX: this is probably okay but it seems slightly dangerous to
	// assume that any "error" we get there is coming from rpmmd, can
	// we generate a more typed error from dnfjson here for rpmmd errors?
	assert.EqualError(t, err, "some error")
	assert.Equal(t, `Code: 23, Reason: rpmmd error in depsolve job, Details: some error`, clientErr.String())
}

func TestWorkerClientErrorFromNil(t *testing.T) {
	clientErr, err := worker.WorkerClientErrorFrom(nil)
	assert.EqualError(t, err, "workerClientErrorFrom expected an error to be processed. Not nil")
	assert.Equal(t, `Code: 23, Reason: rpmmd error in depsolve job, Details: <nil>`, clientErr.String())
}
