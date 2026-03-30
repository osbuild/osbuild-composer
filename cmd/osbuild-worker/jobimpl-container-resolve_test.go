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

func assertResolveResult(t *testing.T, raw json.RawMessage, assertFn func(t *testing.T, cntResolveResult worker.ContainerResolveJobResult)) {
	t.Helper()
	var r worker.ContainerResolveJobResult
	require.NoError(t, json.Unmarshal(raw, &r))
	assertFn(t, r)
}

func TestContainerResolveJobRun(t *testing.T) {
	assertNoopResolveResult := func(t *testing.T, raw json.RawMessage) {
		assertResolveResult(t, raw, func(t *testing.T, cntResolveResult worker.ContainerResolveJobResult) {
			assert.Nil(t, cntResolveResult.JobError)
			assert.Empty(t, cntResolveResult.PipelineSpecs)
		})
	}

	tests := []struct {
		name               string
		jobArgs            *worker.ContainerResolveJob
		jobArgsRaw         json.RawMessage // if jobArgs is nil, use this instead
		finishErr          error
		wantRunErr         bool
		wantErrSubstr      string
		wantFinishCalled   bool
		verifyFinishResult func(t *testing.T, raw json.RawMessage)
	}{
		{
			name: "empty pipeline specs - no-op",
			jobArgs: &worker.ContainerResolveJob{
				Arch:          "x86_64",
				PipelineSpecs: map[string][]worker.ContainerSpec{},
			},
			wantFinishCalled:   true,
			verifyFinishResult: assertNoopResolveResult,
		},
		{
			name: "nil pipeline specs - no-op",
			jobArgs: &worker.ContainerResolveJob{
				Arch:          "x86_64",
				PipelineSpecs: nil,
			},
			wantFinishCalled:   true,
			verifyFinishResult: assertNoopResolveResult,
		},
		{
			name:             "args unmarshal error",
			jobArgsRaw:       json.RawMessage(`{invalid json`),
			wantRunErr:       true,
			wantFinishCalled: true,
		},
		{
			name: "finish error is logged not returned",
			jobArgs: &worker.ContainerResolveJob{
				Arch:          "x86_64",
				PipelineSpecs: map[string][]worker.ContainerSpec{},
			},
			finishErr:        fmt.Errorf("connection lost"),
			wantRunErr:       false,
			wantFinishCalled: true,
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
			wantRunErr:       true,
			wantErrSubstr:    "Error resolving containers for pipeline \"image\":",
			wantFinishCalled: true,
			verifyFinishResult: func(t *testing.T, raw json.RawMessage) {
				assertResolveResult(t, raw, func(t *testing.T, cntResolveResult worker.ContainerResolveJobResult) {
					assert.NotNil(t, cntResolveResult.JobError, "expected job error for unresolvable container")
					assert.Equal(t, clienterrors.ErrorContainerResolution, cntResolveResult.JobError.ID)
				})
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
			wantRunErr:       true,
			wantErrSubstr:    "Error resolving containers for pipeline \"\":",
			wantFinishCalled: true,
			verifyFinishResult: func(t *testing.T, raw json.RawMessage) {
				assertResolveResult(t, raw, func(t *testing.T, cntResolveResult worker.ContainerResolveJobResult) {
					assert.NotNil(t, cntResolveResult.JobError, "expected job error for unresolvable container")
					assert.Equal(t, clienterrors.ErrorContainerResolution, cntResolveResult.JobError.ID)
				})
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

			jobMock := newMockJob(t, worker.JobTypeContainerResolve, rawArgs)
			jobMock.finishErr = tt.finishErr

			impl := &main.ContainerResolveJobImpl{AuthFilePath: ""}
			runErr := impl.Run(jobMock)

			if tt.wantRunErr {
				require.Error(t, runErr)
				if tt.wantErrSubstr != "" {
					assert.Contains(t, runErr.Error(), tt.wantErrSubstr)
				}
			} else {
				require.NoError(t, runErr)
			}

			assert.Equal(t, tt.wantFinishCalled, jobMock.finishCalled, "Finish() called state")
			if tt.verifyFinishResult != nil && jobMock.finishCalled && tt.finishErr == nil {
				tt.verifyFinishResult(t, jobMock.finishResult)
			}
		})
	}
}
