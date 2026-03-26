package main_test

import (
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// mockJob is a minimal worker.Job implementation for unit-testing JobImpl.Run
// methods. It captures the result passed to Finish() so tests can inspect it.
type mockJob struct {
	jobID       uuid.UUID
	jobType     string
	rawArgs     json.RawMessage
	dynamicArgs []json.RawMessage
	finishErr   error

	finishResult json.RawMessage
	finishCalled bool
}

func (j *mockJob) Id() uuid.UUID {
	return j.jobID
}

func (j *mockJob) Type() string {
	return j.jobType
}

func (j *mockJob) Args(args interface{}) error {
	return json.Unmarshal(j.rawArgs, args)
}

func (j *mockJob) DynamicArgs(i int, args interface{}) error {
	if i >= len(j.dynamicArgs) {
		return fmt.Errorf("dynamic args index %d out of range (have %d)", i, len(j.dynamicArgs))
	}
	return json.Unmarshal(j.dynamicArgs[i], args)
}

func (j *mockJob) NDynamicArgs() int {
	return len(j.dynamicArgs)
}

func (j *mockJob) Update(interface{}) error {
	return nil
}

func (j *mockJob) Canceled() (bool, error) {
	return false, nil
}

func (j *mockJob) UploadArtifact(string, io.ReadSeeker) error {
	return nil
}

func (j *mockJob) Finish(result interface{}) error {
	j.finishCalled = true
	if j.finishErr != nil {
		return j.finishErr
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result in mock: %v", err)
	}
	j.finishResult = resultJSON
	return nil
}

func newMockJob(t *testing.T, jobType string, rawArgs json.RawMessage, dynamicArgs ...interface{}) *mockJob {
	t.Helper()
	rawDynArgs := make([]json.RawMessage, len(dynamicArgs))
	for i, da := range dynamicArgs {
		raw, err := json.Marshal(da)
		require.NoError(t, err, "failed to marshal dynamic arg %d", i)
		rawDynArgs[i] = raw
	}
	return &mockJob{
		jobID:       uuid.New(),
		jobType:     jobType,
		rawArgs:     rawArgs,
		dynamicArgs: rawDynArgs,
	}
}

func marshalJobArgs(t *testing.T, args interface{}) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(args)
	require.NoError(t, err)
	return raw
}
