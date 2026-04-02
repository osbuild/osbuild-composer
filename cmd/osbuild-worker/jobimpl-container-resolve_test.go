package main_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	main "github.com/osbuild/osbuild-composer/cmd/osbuild-worker"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

func TestContainerResolveJobRun(t *testing.T) {
	assertEmptyResult := func(t *testing.T, result worker.ContainerResolveJobResult) {
		assert.Nil(t, result.JobError)
		assert.Empty(t, result.PipelineSpecs)
	}

	tests := []struct {
		name               string
		jobArgs            *worker.ContainerResolveJob
		jobArgsRaw         json.RawMessage                                                      // if jobArgs is nil, use this instead
		mockMockJobFunc    func(t *testing.T, jobType string, rawArgs json.RawMessage) *mockJob // if nil, use the default mock job creator
		wantRunErrSubstr   string                                                               // if empty, no error is expected
		verifyFinishResult func(t *testing.T, result worker.ContainerResolveJobResult)
	}{
		{
			name: "empty pipeline specs - no-op",
			jobArgs: &worker.ContainerResolveJob{
				Arch:          "x86_64",
				PipelineSpecs: map[string][]worker.ContainerSpec{},
			},
			verifyFinishResult: assertEmptyResult,
		},
		{
			name: "nil pipeline specs - no-op",
			jobArgs: &worker.ContainerResolveJob{
				Arch:          "x86_64",
				PipelineSpecs: nil,
			},
			verifyFinishResult: assertEmptyResult,
		},
		{
			name:             "args unmarshal error",
			jobArgsRaw:       json.RawMessage(`{invalid json`),
			wantRunErrSubstr: "Error parsing container resolve job args: invalid character 'i' looking for beginning of object key string",
			verifyFinishResult: func(t *testing.T, result worker.ContainerResolveJobResult) {
				assert.NotNil(t, result.JobError)
				assert.Equal(t, clienterrors.ErrorParsingJobArgs, result.JobError.ID)
			},
		},
		{
			name: "job.Finish() error is logged, but not returned from impl.Run()",
			jobArgs: &worker.ContainerResolveJob{
				Arch:          "x86_64",
				PipelineSpecs: map[string][]worker.ContainerSpec{},
			},
			mockMockJobFunc: func(t *testing.T, jobType string, rawArgs json.RawMessage) *mockJob {
				jobMock := newMockJob(t, jobType, rawArgs)
				jobMock.finishErr = fmt.Errorf("connection lost")
				return jobMock
			},
			wantRunErrSubstr: "", // no error expected
		},
		{
			name: "pipeline specs with unresolvable container",
			jobArgs: &worker.ContainerResolveJob{
				Arch: "x86_64",
				PipelineSpecs: map[string][]worker.ContainerSpec{
					"image": {
						{
							Source: "localhost:1/nonexistent/image:latest",
							Name:   "test-container",
						},
					},
				},
			},
			wantRunErrSubstr: "Error resolving containers for pipeline \"image\":",
			verifyFinishResult: func(t *testing.T, result worker.ContainerResolveJobResult) {
				assert.NotNil(t, result.JobError, "expected job error for unresolvable container")
				assert.Equal(t, clienterrors.ErrorContainerResolution, result.JobError.ID)
			},
		},
		{
			name: "old format flat specs - handled via UnmarshalJSON",
			jobArgsRaw: json.RawMessage(`{
				"arch": "x86_64",
				"specs": [
					{
						"source": "localhost:1/nonexistent/image:latest",
						"name": "test-container",
						"image_id": "",
						"digest": ""
					}
				]
			}`),
			wantRunErrSubstr: "Error resolving containers for pipeline \"\":",
			verifyFinishResult: func(t *testing.T, result worker.ContainerResolveJobResult) {
				assert.NotNil(t, result.JobError, "expected job error for unresolvable container")
				assert.Equal(t, clienterrors.ErrorContainerResolution, result.JobError.ID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rawArgs json.RawMessage
			if tt.jobArgs != nil {
				rawArgs = marshalJobArgs(t, *tt.jobArgs)
			} else {
				rawArgs = tt.jobArgsRaw
			}

			// if no mock worker job constructor is provided, use the default mock job creator
			if tt.mockMockJobFunc == nil {
				tt.mockMockJobFunc = newMockJob
			}
			jobMock := tt.mockMockJobFunc(t, worker.JobTypeContainerResolve, rawArgs)

			impl := &main.ContainerResolveJobImpl{AuthFilePath: ""}
			runErr := impl.Run(jobMock)

			if tt.wantRunErrSubstr != "" {
				require.Error(t, runErr)
				assert.Contains(t, runErr.Error(), tt.wantRunErrSubstr)
			} else {
				require.NoError(t, runErr)
			}

			assert.True(t, jobMock.finishCalled, "Finish() should be called")
			if tt.verifyFinishResult != nil {
				var result worker.ContainerResolveJobResult
				require.NoError(t, json.Unmarshal(jobMock.finishResult, &result))
				tt.verifyFinishResult(t, result)
			} else {
				assert.Nil(t, jobMock.finishResult)
			}
		})
	}
}
