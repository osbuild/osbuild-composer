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
	// XXX: this is duplicating the details, see https://github.com/osbuild/images/issues/727
	assert.Equal(t, clientErr.String(), `Code: 20, Reason: DNF error occurred: DepsolveError: something is terribly wrong, Details: something is terribly wrong`)
}

func TestWorkerClientErrorFromOtherError(t *testing.T) {
	otherErr := fmt.Errorf("some error")
	clientErr, err := worker.WorkerClientErrorFrom(otherErr)
	// XXX: this is probably okay but it seems slightly dangerous to
	// assume that any "error" we get there is coming from rpmmd, can
	// we generate a more typed error from dnfjson here for rpmmd errors?
	assert.EqualError(t, err, "rpmmd error in depsolve job: some error")
	assert.Equal(t, clientErr.String(), `Code: 23, Reason: rpmmd error in depsolve job: some error, Details: <nil>`)
}

func TestWorkerClientErrorFromNil(t *testing.T) {
	clientErr, err := worker.WorkerClientErrorFrom(nil)
	// XXX: this is wrong, it should generate an internal error
	assert.EqualError(t, err, "rpmmd error in depsolve job: <nil>")
	assert.Equal(t, clientErr.String(), `Code: 23, Reason: rpmmd error in depsolve job: <nil>, Details: <nil>`)
}
