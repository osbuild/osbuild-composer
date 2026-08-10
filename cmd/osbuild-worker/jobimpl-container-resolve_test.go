package main_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/container"
	main "github.com/osbuild/osbuild-composer/cmd/osbuild-worker"
	"github.com/osbuild/osbuild-composer/internal/common"
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
		jobArgsRaw         json.RawMessage // if jobArgs is nil, use this instead
		dynArgs            []interface{}
		mockMockJobFunc    func(t *testing.T, jobType string, rawArgs json.RawMessage, dynamicArgs ...interface{}) *mockJob // if nil, use the default mock job creator
		wantRunErrSubstr   string                                                                                           // if empty, no error is expected
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
			mockMockJobFunc: func(t *testing.T, jobType string, rawArgs json.RawMessage, dynamicArgs ...interface{}) *mockJob {
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
		{
			name: "PipelineSpecs and PreManifestDynArgsIdx set at the same time",
			jobArgs: &worker.ContainerResolveJob{
				Arch: "x86_64",
				PipelineSpecs: map[string][]worker.ContainerSpec{
					"build": {
						{
							Source: "localhost:1/nonexistent/image:latest",
							Name:   "test-container",
						},
					},
				},
				PreManifestDynArgsIdx: common.ToPtr(0),
			},
			dynArgs: []interface{}{
				worker.BootcPreManifestJobResult{
					ContainerResolveJobArgs: &worker.ContainerResolveJob{
						Arch: "x86_64",
						PipelineSpecs: map[string][]worker.ContainerSpec{
							"build": {
								{
									Source: "localhost:1/nonexistent/image:latest",
									Name:   "test-from-dynargs",
								},
							},
						},
					},
				},
			},
			wantRunErrSubstr: "PipelineSpecs and PreManifestDynArgsIdx cannot be set at the same time",
			verifyFinishResult: func(t *testing.T, result worker.ContainerResolveJobResult) {
				assert.NotNil(t, result.JobError, "expected job error for invalid config")
			},
		},
		{
			name: "dynArgs with PreManifestDynArgsIdx reads specs from BootcPreManifestJobResult",
			jobArgs: &worker.ContainerResolveJob{
				Arch:                  "x86_64",
				PipelineSpecs:         nil,
				PreManifestDynArgsIdx: common.ToPtr(0),
			},
			dynArgs: []interface{}{
				worker.BootcPreManifestJobResult{
					ContainerResolveJobArgs: &worker.ContainerResolveJob{
						Arch: "x86_64",
						PipelineSpecs: map[string][]worker.ContainerSpec{
							"build": {
								{
									Source: "localhost:1/nonexistent/image:latest",
									Name:   "test-from-dynargs",
								},
							},
						},
					},
				},
			},
			wantRunErrSubstr: "Error resolving containers",
			verifyFinishResult: func(t *testing.T, result worker.ContainerResolveJobResult) {
				assert.NotNil(t, result.JobError, "expected job error for unresolvable container from dynArgs")
				assert.Equal(t, clienterrors.ErrorContainerResolution, result.JobError.ID, "expected ErrorContainerResolution, got %d", result.JobError.ID)
			},
		},
		{
			name: "dynArgs with empty ContainerResolveJobArgs is no-op",
			jobArgs: &worker.ContainerResolveJob{
				Arch:                  "x86_64",
				PipelineSpecs:         nil,
				PreManifestDynArgsIdx: common.ToPtr(0),
			},
			dynArgs: []interface{}{worker.BootcPreManifestJobResult{}},
			verifyFinishResult: func(t *testing.T, result worker.ContainerResolveJobResult) {
				assert.Nil(t, result.JobError, "expected no job error for empty ContainerResolveJobArgs")
				assert.Empty(t, result.PipelineSpecs, "expected empty result PipelineSpecs when ContainerResolveJobArgs is empty")
			},
		},
		{
			name: "dynArgs index out of range",
			jobArgs: &worker.ContainerResolveJob{
				Arch:                  "x86_64",
				PipelineSpecs:         nil,
				PreManifestDynArgsIdx: common.ToPtr(5),
			},
			wantRunErrSubstr: "Error reading container resolve args from dynamic args",
			verifyFinishResult: func(t *testing.T, result worker.ContainerResolveJobResult) {
				require.NotNil(t, result.JobError, "expected job error for out-of-range dynArgs index")
				assert.Equal(t, clienterrors.ErrorParsingDynamicArgs, result.JobError.ID, "expected ErrorParsingDynamicArgs, got %d", result.JobError.ID)
			},
		},
		{
			name: "dynArgs index out of range - negative index",
			jobArgs: &worker.ContainerResolveJob{
				Arch:                  "x86_64",
				PipelineSpecs:         nil,
				PreManifestDynArgsIdx: common.ToPtr(-1),
			},
			wantRunErrSubstr: "Error reading container resolve args from dynamic args",
			verifyFinishResult: func(t *testing.T, result worker.ContainerResolveJobResult) {
				require.NotNil(t, result.JobError, "expected job error for out-of-range dynArgs index")
				assert.Equal(t, clienterrors.ErrorParsingDynamicArgs, result.JobError.ID, "expected ErrorParsingDynamicArgs, got %d", result.JobError.ID)
			},
		},
		{
			name: "dynArgs dependency failed",
			jobArgs: &worker.ContainerResolveJob{
				Arch:                  "x86_64",
				PipelineSpecs:         nil,
				PreManifestDynArgsIdx: common.ToPtr(0),
			},
			dynArgs: []interface{}{
				worker.BootcPreManifestJobResult{
					JobResult: worker.JobResult{
						JobError: clienterrors.New(clienterrors.ErrorBuildJob, "pre-manifest failed", nil),
					},
				},
			},
			wantRunErrSubstr: "Error reading container resolve args from dynamic args",
			verifyFinishResult: func(t *testing.T, result worker.ContainerResolveJobResult) {
				require.NotNil(t, result.JobError, "expected job error for failed dependency")
				assert.Equal(t, clienterrors.ErrorJobDependency, result.JobError.ID, "expected ErrorJobDependency, got %d", result.JobError.ID)
			},
		},
		{
			name: "no dynArgs without PreManifestDynArgsIdx - standard flow unchanged",
			jobArgs: &worker.ContainerResolveJob{
				Arch: "x86_64",
				PipelineSpecs: map[string][]worker.ContainerSpec{
					"build": {
						{
							Source: "localhost:1/nonexistent/image:latest",
							Name:   "test-container",
						},
					},
				},
				PreManifestDynArgsIdx: nil,
			},
			wantRunErrSubstr: "Error resolving containers",
			verifyFinishResult: func(t *testing.T, result worker.ContainerResolveJobResult) {
				assert.NotNil(t, result.JobError, "expected job error for unresolvable container")
				assert.Equal(t, clienterrors.ErrorContainerResolution, result.JobError.ID, "expected ErrorContainerResolution, got %d", result.JobError.ID)
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
			jobMock := tt.mockMockJobFunc(t, worker.JobTypeContainerResolve, rawArgs, tt.dynArgs...)

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

// mockResolver implements container.Resolver by returning a fixed digest
// for each source, allowing tests to verify ordering without a registry.
type mockResolver struct {
	digests map[string]string
}

func (m *mockResolver) Add(container.SourceSpec) {}

func (m *mockResolver) SetAuthFilePath(string) {}

func (m *mockResolver) Finish() ([]container.Spec, error) {
	return nil, fmt.Errorf("mockResolver: use Resolve()")
}

func (m *mockResolver) Resolve(src container.SourceSpec) (container.Spec, error) {
	digest, ok := m.digests[src.Source]
	if !ok {
		return container.Spec{}, fmt.Errorf("unknown source %q", src.Source)
	}
	return container.Spec{
		Source:    src.Source,
		LocalName: src.Name,
		Digest:    digest,
	}, nil
}

func (m *mockResolver) ResolveAll(sources map[string][]container.SourceSpec) (map[string][]container.Spec, error) {
	return nil, fmt.Errorf("mockResolver: use Resolve()")
}

func TestResolveContainersPreservesOrder(t *testing.T) {
	resolver := &mockResolver{
		digests: map[string]string{
			"quay.io/os-tree:latest": "sha256:zzz",
			"quay.io/payload:latest": "sha256:aaa",
		},
	}

	pipelineSpecs := map[string][]worker.ContainerSpec{
		"os-tree": {
			{Source: "quay.io/os-tree:latest", Name: "quay.io/os-tree:latest"},
			{Source: "quay.io/payload:latest", Name: "quay.io/payload:latest"},
		},
		"build": {
			{Source: "quay.io/os-tree:latest", Name: "quay.io/os-tree:latest"},
		},
	}

	result, err := main.ResolveContainers(resolver, pipelineSpecs)
	require.NoError(t, err)

	// os-tree pipeline: order must match input, not digest order
	require.Len(t, result["os-tree"], 2)
	assert.Equal(t, "quay.io/os-tree:latest", result["os-tree"][0].Source)
	assert.Equal(t, "sha256:zzz", result["os-tree"][0].Digest)
	assert.Equal(t, "quay.io/payload:latest", result["os-tree"][1].Source)
	assert.Equal(t, "sha256:aaa", result["os-tree"][1].Digest)

	// build pipeline: single entry
	require.Len(t, result["build"], 1)
	assert.Equal(t, "quay.io/os-tree:latest", result["build"][0].Source)
}
