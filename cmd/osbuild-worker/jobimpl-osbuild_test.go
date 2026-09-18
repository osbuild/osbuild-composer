package main_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/osbuild"

	main "github.com/osbuild/osbuild-composer/cmd/osbuild-worker"
	"github.com/osbuild/osbuild-composer/internal/osbuildexecutor"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

func TestMakeJobErrorFromOsbuildOutput(t *testing.T) {
	tests := []struct {
		inputData *osbuild.Result
		expected  string
	}{
		{
			inputData: &osbuild.Result{
				Success: false,
				Log: map[string]osbuild.PipelineResult{
					"fake-os": []osbuild.StageResult{
						{
							Type:    "good-stage",
							Success: true,
							Output:  "good-output",
						},
						{
							Type:    "bad-stage",
							Success: false,
							Output:  "bad-failure",
						},
					},
				},
			},
			expected: `Code: 10, Reason: build failure, Details: [bad-failure]`,
		},
		{
			inputData: &osbuild.Result{
				Success: false,
				Log: map[string]osbuild.PipelineResult{
					"fake-os": []osbuild.StageResult{},
				},
			},
			expected: `Code: 10, Reason: build failure, Details: []`,
		},
		{
			inputData: &osbuild.Result{
				Error:   json.RawMessage("some_osbuild_error"),
				Success: false,
				Log: map[string]osbuild.PipelineResult{
					"fake-os": []osbuild.StageResult{},
				},
			},
			expected: `Code: 10, Reason: build failure, Details: [some_osbuild_error]`,
		},
		{
			inputData: &osbuild.Result{
				Errors: []osbuild.ValidationError{
					{
						Message: "validation error message",
						Path:    []string{"the", "error", "path"},
					},
				},
				Success: false,
				Log: map[string]osbuild.PipelineResult{
					"fake-os": []osbuild.StageResult{},
				},
			},
			expected: `Code: 10, Reason: build failure, Details: [validation error in the.error.path: validation error message]`,
		},
	}
	for _, testData := range tests {
		fakeOsbuildResult := testData.inputData

		wce := main.MakeJobErrorFromOsbuildOutput(fakeOsbuildResult)
		require.Equal(t, testData.expected, wce.String())
	}
}

func TestJobErrorFromOSBuildRun(t *testing.T) {
	gone := fmt.Errorf("%w: connection refused", osbuildexecutor.ErrSecureInstanceGone)
	err := main.JobErrorFromOSBuildRun(gone)
	require.Equal(t, clienterrors.ErrorSecureInstance, err.ID)
	require.Equal(t, "secure instance died", err.Reason)
	require.Equal(t, gone.Error(), err.Details)

	other := errors.New("osbuild failed: log")
	err = main.JobErrorFromOSBuildRun(other)
	require.Equal(t, clienterrors.ErrorBuildJob, err.ID)
	require.Equal(t, "osbuild failed", err.Reason)
	require.Equal(t, other.Error(), err.Details)
}
