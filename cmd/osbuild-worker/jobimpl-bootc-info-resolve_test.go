package main_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/bib/osinfo"
	"github.com/osbuild/images/pkg/bootc"
	main "github.com/osbuild/osbuild-composer/cmd/osbuild-worker"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

func mockResolveBootcInfoFunc(t *testing.T, resolveMode worker.BootcInfoResolveMode) main.ResolveBootcInfoFuncType {
	t.Helper()

	baseInfo := bootc.Info{
		ImageID:       "sha256:abc123",
		Arch:          "x86_64",
		DefaultRootFs: "xfs",
		Size:          1073741824,
	}

	// NOTE: if the consumer of the info or any of the closures below starts
	// to modify the OSInfo struct or any of its composite fields, we need to
	// make a copy of the struct here.
	baseOSInfo := &osinfo.Info{
		OSRelease: osinfo.OSRelease{
			ID:        "centos",
			VersionID: "9",
		},
		KernelInfo: &osinfo.KernelInfo{
			Version: "5.10.0",
		},
	}

	switch resolveMode {
	case worker.BootcInfoResolveModeFull:
		return func(ref string) (*bootc.Info, error) {
			info := baseInfo
			info.Imgref = ref
			info.OSInfo = baseOSInfo
			return &info, nil
		}
	case worker.BootcInfoResolveModeBuild:
		return func(ref string) (*bootc.Info, error) {
			info := baseInfo
			info.Imgref = ref
			return &info, nil
		}
	default:
		panic(fmt.Sprintf("invalid resolve mode: %s", resolveMode))
	}
}

func TestBootcInfoResolveJobRun(t *testing.T) {
	assertEmptyResult := func(t *testing.T, result worker.BootcInfoResolveJobResult) {
		assert.Nil(t, result.JobError)
		assert.Empty(t, result.Infos)
	}

	tests := []struct {
		name                 string
		jobArgs              *worker.BootcInfoResolveJob
		jobArgsRaw           json.RawMessage
		mockResolveFullInfo  main.ResolveBootcInfoFuncType                                                                    // if nil, use the default mock function
		mockResolveBuildInfo main.ResolveBootcInfoFuncType                                                                    // if nil, use the default mock function
		mockMockJobFunc      func(t *testing.T, jobType string, rawArgs json.RawMessage, dynamicArgs ...interface{}) *mockJob // if nil, use the default mock job creator
		wantRunErrSubstr     string                                                                                           // if empty, no error is expected
		verifyFinishResult   func(t *testing.T, result worker.BootcInfoResolveJobResult)
	}{
		{
			name: "empty specs - no-op",
			jobArgs: &worker.BootcInfoResolveJob{
				Specs: []worker.BootcInfoResolveJobSpec{},
			},
			verifyFinishResult: assertEmptyResult,
		},
		{
			name: "nil specs - no-op",
			jobArgs: &worker.BootcInfoResolveJob{
				Specs: nil,
			},
			verifyFinishResult: assertEmptyResult,
		},
		{
			name:             "args unmarshal error",
			jobArgsRaw:       json.RawMessage(`{invalid json`),
			wantRunErrSubstr: "Error parsing bootc info resolve job args:",
			verifyFinishResult: func(t *testing.T, result worker.BootcInfoResolveJobResult) {
				assert.NotNil(t, result.JobError)
				assert.Equal(t, clienterrors.ErrorParsingJobArgs, result.JobError.ID)
			},
		},
		{
			name: "invalid resolve mode",
			jobArgsRaw: json.RawMessage(`{
				"specs": [
					{
						"reference": "localhost:1/nonexistent/image:latest",
						"resolve_mode": "invalid-mode"
					}
				]
			}`),
			wantRunErrSubstr: "Error parsing bootc info resolve job args: invalid bootc info resolve mode: \"invalid-mode\"",
			verifyFinishResult: func(t *testing.T, result worker.BootcInfoResolveJobResult) {
				assert.NotNil(t, result.JobError)
				assert.Equal(t, clienterrors.ErrorParsingJobArgs, result.JobError.ID)
			},
		},
		{
			name: "full resolve mode with unresolvable container",
			jobArgs: &worker.BootcInfoResolveJob{
				Specs: []worker.BootcInfoResolveJobSpec{
					{
						Ref:         "localhost:1/nonexistent/image:latest",
						ResolveMode: worker.BootcInfoResolveModeFull,
					},
				},
			},
			mockResolveFullInfo: func(ref string) (*bootc.Info, error) {
				return nil, fmt.Errorf("pull failed for ref %s", ref)
			},
			wantRunErrSubstr: "pull failed for ref",
			verifyFinishResult: func(t *testing.T, result worker.BootcInfoResolveJobResult) {
				assert.NotNil(t, result.JobError, "expected job error for unresolvable container")
				assert.Equal(t, clienterrors.ErrorBootcInfoResolve, result.JobError.ID)
			},
		},
		{
			name: "build resolve mode with unresolvable container",
			jobArgs: &worker.BootcInfoResolveJob{
				Specs: []worker.BootcInfoResolveJobSpec{
					{Ref: "localhost:1/nonexistent/image:latest", ResolveMode: worker.BootcInfoResolveModeBuild},
				},
			},
			mockResolveBuildInfo: func(ref string) (*bootc.Info, error) {
				return nil, fmt.Errorf("pull failed for ref %s", ref)
			},
			wantRunErrSubstr: "pull failed for ref",
			verifyFinishResult: func(t *testing.T, result worker.BootcInfoResolveJobResult) {
				assert.NotNil(t, result.JobError, "expected job error for unresolvable container")
				assert.Equal(t, clienterrors.ErrorBootcInfoResolve, result.JobError.ID)
			},
		},
		{
			name: "Happy path: verify that the result order matches the spec order",
			jobArgs: &worker.BootcInfoResolveJob{
				Specs: []worker.BootcInfoResolveJobSpec{
					{Ref: "localhost:1/some/image:latest", ResolveMode: worker.BootcInfoResolveModeFull},
					{Ref: "localhost:1/another/image:latest", ResolveMode: worker.BootcInfoResolveModeBuild},
					{Ref: "localhost:1/yet-another/image:latest", ResolveMode: worker.BootcInfoResolveModeFull},
					{Ref: "localhost:1/yet-another-2/image:latest", ResolveMode: worker.BootcInfoResolveModeBuild},
				},
			},
			verifyFinishResult: func(t *testing.T, result worker.BootcInfoResolveJobResult) {
				assert.Nil(t, result.JobError)
				assert.Len(t, result.Infos, 4)
				assert.Equal(t, "localhost:1/some/image:latest", result.Infos[0].Imgref)
				assert.Equal(t, "localhost:1/another/image:latest", result.Infos[1].Imgref)
				assert.Equal(t, "localhost:1/yet-another/image:latest", result.Infos[2].Imgref)
				assert.Equal(t, "localhost:1/yet-another-2/image:latest", result.Infos[3].Imgref)
			},
		},
		{
			name: "job.Finish() error is logged, but not returned from impl.Run()",
			jobArgs: &worker.BootcInfoResolveJob{
				Specs: []worker.BootcInfoResolveJobSpec{},
			},
			mockMockJobFunc: func(t *testing.T, jobType string, rawArgs json.RawMessage, dynamicArgs ...interface{}) *mockJob {
				jobMock := newMockJob(t, jobType, rawArgs)
				jobMock.finishErr = fmt.Errorf("connection lost")
				return jobMock
			},
			wantRunErrSubstr: "", // no error expected
		},
		{
			name: "resolver error on second spec returns JobError and no result",
			jobArgs: &worker.BootcInfoResolveJob{
				Specs: []worker.BootcInfoResolveJobSpec{
					{Ref: "localhost:1/some/image:latest", ResolveMode: worker.BootcInfoResolveModeFull},
					{Ref: "localhost:1/another/image:latest", ResolveMode: worker.BootcInfoResolveModeBuild},
				},
			},
			mockResolveFullInfo: nil, // use the default mock function, which returns a resolved bootc.Info
			mockResolveBuildInfo: func(ref string) (*bootc.Info, error) {
				return nil, fmt.Errorf("pull failed for ref %s", ref)
			},
			wantRunErrSubstr: "pull failed for ref",
			verifyFinishResult: func(t *testing.T, result worker.BootcInfoResolveJobResult) {
				assert.NotNil(t, result.JobError, "expected job error for unresolvable container")
				assert.Nil(t, result.Infos)

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
			jobMock := tt.mockMockJobFunc(t, worker.JobTypeBootcInfoResolve, rawArgs)

			if tt.mockResolveFullInfo == nil {
				tt.mockResolveFullInfo = mockResolveBootcInfoFunc(t, worker.BootcInfoResolveModeFull)
			}
			if tt.mockResolveBuildInfo == nil {
				tt.mockResolveBuildInfo = mockResolveBootcInfoFunc(t, worker.BootcInfoResolveModeBuild)
			}
			t.Cleanup(main.MockResolveBootcInfoFunc(tt.mockResolveFullInfo))
			t.Cleanup(main.MockResolveBootcBuildInfoFunc(tt.mockResolveBuildInfo))

			impl := &main.BootcInfoResolveJobImpl{}
			runErr := impl.Run(jobMock)

			if tt.wantRunErrSubstr != "" {
				require.Error(t, runErr)
				assert.Contains(t, runErr.Error(), tt.wantRunErrSubstr)
			} else {
				require.NoError(t, runErr)
			}

			assert.True(t, jobMock.finishCalled, "Finish() should be called")
			if tt.verifyFinishResult != nil {
				var result worker.BootcInfoResolveJobResult
				require.NoError(t, json.Unmarshal(jobMock.finishResult, &result))
				tt.verifyFinishResult(t, result)
			} else {
				assert.Nil(t, jobMock.finishResult)
			}
		})
	}
}
