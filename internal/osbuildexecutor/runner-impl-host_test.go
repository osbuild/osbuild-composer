package osbuildexecutor_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/osbuild"

	"github.com/osbuild/osbuild-composer/internal/osbuildexecutor"
)

func TestHostRunOSBuild(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("PATH", tmpDir)
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	tests := []struct {
		name    string
		osbuild string
		json    bool
		error   string
		result  *osbuild.Result
	}{
		{
			name:    "jsonoutput: invalid json results in error with raw output",
			osbuild: "echo random output",
			json:    true,
			error:   "error decoding osbuild output: invalid character 'r' looking for beginning of value\nraw output:\nrandom output\n",
		},
		{
			name: "jsonoutput: invalid json with exit code 1 results in error",
			osbuild: `echo random output
exit 1
`,
			json:  true,
			error: "error decoding osbuild output: invalid character 'r' looking for beginning of value\nraw output:\nrandom output\n",
		},
		{
			name: "jsonoutput: exit code 1 still returns valid results",
			osbuild: `echo '{"success":false,"log":{"pipeline":[{"id":"stage","success":false,"output":"pigeon"}]}}'
exit 1
`,
			json: true,
			result: &osbuild.Result{
				Success: false,
				Log: map[string]osbuild.PipelineResult{
					"pipeline": {
						{
							ID:      "stage",
							Success: false,
							Output:  "pigeon",
						},
					},
				},
			},
		},
		{
			name: "jsonoutput: exit code 2 (validation errors) still returns valid results",
			osbuild: `echo '{"success":false,"errors":[{"message":"pigeon","path":["path","to","pigeon"]}]}'
exit 1
`,
			json: true,
			result: &osbuild.Result{
				Success: false,
				Errors: []osbuild.ValidationError{
					{
						Message: "pigeon",
						Path:    []string{"path", "to", "pigeon"},
					},
				},
			},
		},
		{
			name: "jsonoutput: exit code 2 (validation errors) still returns valid results",
			osbuild: `echo '{"success":false,"errors":[{"message":"pigeon","path":["path","to","pigeon"]}]}'
exit 1
`,
			json: true,
			result: &osbuild.Result{
				Success: false,
				Errors: []osbuild.ValidationError{
					{
						Message: "pigeon",
						Path:    []string{"path", "to", "pigeon"},
					},
				},
			},
		},
		{
			name: "no json output: valid",
			osbuild: `echo 'worked'
exit 0
`,
			result: &osbuild.Result{
				Success: true,
			},
			json: false,
		},
		{
			name: "no json output: exit code 1",
			osbuild: `echo 'some error'
exit 1
`,
			json:  false,
			error: "osbuild failed: exit status 1, some error\n",
		},
		{
			name: "no json output: exit code 2",
			osbuild: `echo 'some validation error'
exit 2
`,
			json:  false,
			error: "osbuild failed: exit status 2, some validation error\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//nolint:gosec
			err := os.WriteFile(filepath.Join(tmpDir, "osbuild"), []byte(fmt.Sprintf(`#!/bin/sh
%s
`, tt.osbuild)), 0700)
			require.NoError(t, err)

			hostExe := osbuildexecutor.NewHostExecutor()
			result, err := hostExe.RunOSBuild(nil, logger, nil, &osbuild.OSBuildOptions{
				JSONOutput: tt.json,
			})
			if tt.error != "" {
				assert.Error(t, err)
				assert.Equal(t, tt.error, err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.result, result)
		})
	}
}
