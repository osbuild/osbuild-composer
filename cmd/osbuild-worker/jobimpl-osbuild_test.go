package main_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/osbuild"

	main "github.com/osbuild/osbuild-composer/cmd/osbuild-worker"
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
