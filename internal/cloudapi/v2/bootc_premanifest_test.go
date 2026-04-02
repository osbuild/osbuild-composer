package v2_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/bib/osinfo"
	"github.com/osbuild/images/pkg/manifest"
	v2 "github.com/osbuild/osbuild-composer/internal/cloudapi/v2"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

// newTestWorkerServer creates a minimal worker server for testing
// handleBootcPreManifest directly, without the full v2 server stack.
func newTestWorkerServer(t *testing.T) *worker.Server {
	t.Helper()
	jobsDir := filepath.Join(t.TempDir(), "jobs")
	err := os.Mkdir(jobsDir, 0755)
	require.NoError(t, err)

	q, err := fsjobqueue.New(jobsDir)
	require.NoError(t, err)

	return worker.NewServer(nil, q, worker.Config{
		BasePath: "/api/worker/v1",
	})
}

// rawValidBaseBootcInfoResult returns a marshaled BootcInfoResolveJobResult
// matching the test container used across bootc pre-manifest handler tests.
func rawValidBaseBootcInfoResult(t *testing.T) json.RawMessage {
	t.Helper()
	osInfo := &osinfo.Info{
		OSRelease: osinfo.OSRelease{
			ID:        "centos",
			VersionID: "9",
		},
	}
	data, err := json.Marshal(osInfo)
	require.NoError(t, err)

	baseResult := worker.BootcInfoResolveJobResult{
		Infos: []worker.BootcContainerInfo{
			{
				Imgref:        "quay.io/centos-bootc/centos-bootc:stream9",
				ImageID:       "sha256:abc123",
				Arch:          "x86_64",
				DefaultRootFs: "xfs",
				Size:          1073741824,
				OSInfo:        data,
			},
		},
	}
	b, err := json.Marshal(baseResult)
	require.NoError(t, err)

	return b
}

func TestHandleBootcPreManifest_Errors(t *testing.T) {
	tests := []struct {
		name               string
		job                *worker.BootcPreManifestJob
		staticArgsOverride func(t *testing.T) json.RawMessage
		dynArgs            func(t *testing.T) []json.RawMessage
		wantErrID          clienterrors.ClientErrorCode
		wantReasonContains string
	}{
		{
			name: "invalid_static_args_JSON",
			job: &worker.BootcPreManifestJob{
				ImageType:                  "qcow2",
				Seed:                       42,
				BootcInfoResolveDynArgsIdx: common.ToPtr(0),
			},
			staticArgsOverride: func(t *testing.T) json.RawMessage {
				t.Helper()
				return json.RawMessage(`{invalid`)
			},
			wantErrID:          clienterrors.ErrorParsingJobArgs,
			wantReasonContains: "Error parsing bootc pre-manifest job args",
		},
		{
			name: "missing_info_resolve_index",
			job: &worker.BootcPreManifestJob{
				ImageType:                  "qcow2",
				Seed:                       42,
				BootcInfoResolveDynArgsIdx: nil,
			},
			wantErrID:          clienterrors.ErrorParsingDynamicArgs,
			wantReasonContains: "BootcInfoResolveDynArgsIdx is missing or out of range",
		},
		{
			name: "info_resolve_index_out_of_range",
			job: &worker.BootcPreManifestJob{
				ImageType:                  "qcow2",
				Seed:                       42,
				BootcInfoResolveDynArgsIdx: common.ToPtr(5),
			},
			wantErrID:          clienterrors.ErrorParsingDynamicArgs,
			wantReasonContains: "BootcInfoResolveDynArgsIdx is missing or out of range",
		},
		{
			name: "info_resolve_index_out_of_range_negative",
			job: &worker.BootcPreManifestJob{
				ImageType:                  "qcow2",
				Seed:                       42,
				BootcInfoResolveDynArgsIdx: common.ToPtr(-1),
			},
			wantErrID:          clienterrors.ErrorParsingDynamicArgs,
			wantReasonContains: "BootcInfoResolveDynArgsIdx is missing or out of range",
		},
		{
			name: "invalid_info_resolve_dynArg_JSON",
			job: &worker.BootcPreManifestJob{
				ImageType:                  "qcow2",
				Seed:                       42,
				BootcInfoResolveDynArgsIdx: common.ToPtr(0),
			},
			dynArgs: func(t *testing.T) []json.RawMessage {
				t.Helper()
				return []json.RawMessage{json.RawMessage(`{invalid json`)}
			},
			wantErrID:          clienterrors.ErrorParsingDynamicArgs,
			wantReasonContains: "Error parsing bootc info resolve result: invalid character",
		},
		{
			name: "info_resolve_dependency_failed",
			job: &worker.BootcPreManifestJob{
				ImageType:                  "qcow2",
				Seed:                       42,
				BootcInfoResolveDynArgsIdx: common.ToPtr(0),
			},
			dynArgs: func(t *testing.T) []json.RawMessage {
				t.Helper()
				failedResult := worker.BootcInfoResolveJobResult{
					JobResult: worker.JobResult{
						JobError: clienterrors.New(
							clienterrors.ErrorBootcInfoResolve,
							"container not found", nil,
						),
					},
				}
				b, err := json.Marshal(failedResult)
				require.NoError(t, err)
				return []json.RawMessage{b}
			},
			wantErrID:          clienterrors.ErrorManifestGeneration,
			wantReasonContains: "bootc info resolve dependency failed",
		},
		{
			name: "base_index_out_of_range",
			job: &worker.BootcPreManifestJob{
				ImageType:                  "qcow2",
				Seed:                       42,
				BootcInfoResolveDynArgsIdx: common.ToPtr(0),
				BaseInfoIdx:                5,
			},
			dynArgs: func(t *testing.T) []json.RawMessage {
				t.Helper()
				return []json.RawMessage{rawValidBaseBootcInfoResult(t)}
			},
			wantErrID:          clienterrors.ErrorManifestGeneration,
			wantReasonContains: "base info index 5 is out of range (resolved 1 infos)",
		},
		{
			name: "base_info_index_out_of_range_negative",
			job: &worker.BootcPreManifestJob{
				ImageType:                  "qcow2",
				Seed:                       42,
				BootcInfoResolveDynArgsIdx: common.ToPtr(0),
				BaseInfoIdx:                -1,
			},
			dynArgs: func(t *testing.T) []json.RawMessage {
				t.Helper()
				return []json.RawMessage{rawValidBaseBootcInfoResult(t)}
			},
			wantErrID:          clienterrors.ErrorManifestGeneration,
			wantReasonContains: "base info index -1 is out of range (resolved 1 infos)",
		},
		{
			name: "build_index_out_of_range",
			job: &worker.BootcPreManifestJob{
				ImageType:                  "qcow2",
				Seed:                       42,
				BootcInfoResolveDynArgsIdx: common.ToPtr(0),
				BuildInfoIdx:               common.ToPtr(5),
			},
			dynArgs: func(t *testing.T) []json.RawMessage {
				t.Helper()
				return []json.RawMessage{rawValidBaseBootcInfoResult(t)}
			},
			wantErrID:          clienterrors.ErrorManifestGeneration,
			wantReasonContains: "build info index 5 is out of range (resolved 1 infos)",
		},
		{
			name: "build_info_index_out_of_range_negative",
			job: &worker.BootcPreManifestJob{
				ImageType:                  "qcow2",
				Seed:                       42,
				BootcInfoResolveDynArgsIdx: common.ToPtr(0),
				BuildInfoIdx:               common.ToPtr(-1),
			},
			dynArgs: func(t *testing.T) []json.RawMessage {
				t.Helper()
				return []json.RawMessage{rawValidBaseBootcInfoResult(t)}
			},
			wantErrID:          clienterrors.ErrorManifestGeneration,
			wantReasonContains: "build info index -1 is out of range (resolved 1 infos)",
		},
		{
			name: "invalid_image_type",
			job: &worker.BootcPreManifestJob{
				ImageType:                  "nonexistent-image-type",
				Seed:                       42,
				BootcInfoResolveDynArgsIdx: common.ToPtr(0),
			},
			dynArgs: func(t *testing.T) []json.RawMessage {
				t.Helper()
				return []json.RawMessage{rawValidBaseBootcInfoResult(t)}
			},
			wantErrID:          clienterrors.ErrorManifestGeneration,
			wantReasonContains: "Error generating bootc pre-manifest: getting image type \"nonexistent-image-type\": invalid image type: nonexistent-image-type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := newTestWorkerServer(t)
			preManifestJobID, err := ws.EnqueueBootcPreManifestJob(tt.job, nil, "")
			require.NoError(t, err)
			jobID, token, _, _, _, err := ws.RequestJob(
				context.Background(), "",
				[]string{worker.JobTypeBootcPreManifest}, []string{""}, uuid.Nil,
			)
			require.NoError(t, err)

			var staticArgs json.RawMessage
			if tt.staticArgsOverride != nil {
				staticArgs = tt.staticArgsOverride(t)
			} else {
				var err error
				staticArgs, err = json.Marshal(tt.job)
				require.NoError(t, err)
			}

			var dynArgs []json.RawMessage
			if tt.dynArgs != nil {
				dynArgs = tt.dynArgs(t)
			}

			v2.HandleBootcPreManifest(ws, jobID, token, staticArgs, dynArgs)

			var readResult worker.BootcPreManifestJobResult
			jobInfo, err := ws.BootcPreManifestJobInfo(preManifestJobID, &readResult)
			require.NoError(t, err)
			require.NotNil(t, jobInfo)
			assert.False(t, jobInfo.JobStatus.Finished.IsZero(), "job should be finished (defer always calls FinishJob)")
			require.NotNil(t, readResult.JobError)
			assert.Equal(t, tt.wantErrID, readResult.JobError.ID)
			assert.Contains(t, readResult.JobError.Reason, tt.wantReasonContains)
		})
	}
}

// enqueuePreManifestWithResolvedDep enqueues a bootc info-resolve job,
// finishes it with a valid result, and enqueues a pre-manifest job that
// depends on it. Returns the pre-manifest job ID.
func enqueuePreManifestWithResolvedDep(t *testing.T, ws *worker.Server, arch string, specs []worker.BootcInfoResolveJobSpec) uuid.UUID {
	t.Helper()
	infoResolveJob := &worker.BootcInfoResolveJob{
		Specs: specs,
	}
	infoResolveJobID, err := ws.EnqueueBootcInfoResolveJob(arch, infoResolveJob, "")
	require.NoError(t, err)

	preManifestJob := &worker.BootcPreManifestJob{
		ImageType:                  "qcow2",
		Seed:                       42,
		BootcInfoResolveDynArgsIdx: common.ToPtr(0),
	}
	preManifestJobID, err := ws.EnqueueBootcPreManifestJob(
		preManifestJob, []uuid.UUID{infoResolveJobID}, "",
	)
	require.NoError(t, err)

	_, infoToken, _, _, _, err := ws.RequestJob(
		context.Background(), arch,
		[]string{worker.JobTypeBootcInfoResolve}, []string{""}, uuid.Nil,
	)
	require.NoError(t, err)

	err = ws.FinishJob(infoToken, rawValidBaseBootcInfoResult(t))
	require.NoError(t, err)

	return preManifestJobID
}

// assertValidPreManifestResult checks that a BootcPreManifestJobResult
// completed without error and contains the expected container resolve data
// for the test fixture container (centos-bootc:stream9 on x86_64).
func assertValidPreManifestResult(t *testing.T, result worker.BootcPreManifestJobResult, arch string, specs []worker.BootcInfoResolveJobSpec) {
	t.Helper()

	require.Nil(t, result.JobError, "expected no job error, got: %v", result.JobError)

	assert.Equal(t, arch, result.ContainerResolveJobArgs.Arch)
	assert.Len(t, result.ContainerResolveJobArgs.PipelineSpecs, 2, "expected 2 pipelines specs")
	assert.Len(t, result.ContainerResolveJobArgs.PipelineSpecs["image"], 1, "expected 1 container spec in image pipeline")
	assert.Len(t, result.ContainerResolveJobArgs.PipelineSpecs["build"], 1, "expected 1 container spec in build pipeline")

	// Verify that all container refs from BootcInfoResolveJobSpec are present
	// in the container resolve job results for at least one pipeline.
	for _, spec := range specs {
		t.Run(spec.Ref, func(t *testing.T) {
			for _, pipelineSpecs := range result.ContainerResolveJobArgs.PipelineSpecs {
				for _, pipelineSpec := range pipelineSpecs {
					if pipelineSpec.Source == spec.Ref {
						// found the spec in the pipeline, break the inner loop
						return
					}
				}
			}
			assert.Fail(t, "expected container spec with source %s not found", spec.Ref)
		})
	}
}

// TestHandleBootcPreManifest_HappyPath tests the happy path for the
// BootcPreManifest job: enqueue a pre-manifest job with a completed
// dependency, and verify the job finishes successfully. Without going
// through the loop.
func TestHandleBootcPreManifest_HappyPath(t *testing.T) {
	workerServer := newTestWorkerServer(t)
	specs := []worker.BootcInfoResolveJobSpec{
		{
			Ref:         "quay.io/centos-bootc/centos-bootc:stream9",
			ResolveMode: worker.BootcInfoResolveModeFull,
		},
	}
	archi := arch.ARCH_X86_64.String()
	preManifestJobID := enqueuePreManifestWithResolvedDep(t, workerServer, archi, specs)

	// Dequeue the pre-manifest job (it should be pending now)
	jobID, preManifestToken, _, staticArgs, dynArgs, err := workerServer.RequestJob(
		context.Background(), "",
		[]string{worker.JobTypeBootcPreManifest}, []string{""}, uuid.Nil,
	)
	require.NoError(t, err)

	// Call the handler
	v2.HandleBootcPreManifest(workerServer, jobID, preManifestToken, staticArgs, dynArgs)

	// Verify the job finished successfully
	var readResult worker.BootcPreManifestJobResult
	jobInfo, err := workerServer.BootcPreManifestJobInfo(preManifestJobID, &readResult)
	require.NoError(t, err)
	require.NotNil(t, jobInfo)
	assert.False(t, jobInfo.JobStatus.Finished.IsZero(), "job should be finished")

	assertValidPreManifestResult(t, readResult, archi, specs)

	// Verify ManifestInfo is populated
	assert.Equal(t, common.BuildVersion(), readResult.ManifestInfo.OSBuildComposerVersion)
	// OSBuildComposerDeps may be nil in tests. See https://github.com/golang/go/issues/33976
	if readResult.ManifestInfo.OSBuildComposerDeps != nil {
		assert.Len(t, readResult.ManifestInfo.OSBuildComposerDeps, 1)
		assert.Equal(t, "github.com/osbuild/images", readResult.ManifestInfo.OSBuildComposerDeps[0].Path)
		assert.NotEmpty(t, readResult.ManifestInfo.OSBuildComposerDeps[0].Version)
	}
	assert.NotNil(t, readResult.ManifestInfo.PipelineNames)
	assert.NotEmpty(t, readResult.ManifestInfo.PipelineNames.Build)
	assert.NotEmpty(t, readResult.ManifestInfo.PipelineNames.Payload)
}

// TestBootcPreManifestLoop_PicksUpJob tests the full loop lifecycle:
// Start v2 server (which starts the loop), enqueue a pre-manifest
// job with completed dependencies, and verify the job gets finished.
func TestBootcPreManifestLoop_PicksUpJob(t *testing.T) {
	workerServer := newTestWorkerServer(t)

	// Create v2 server which starts the bootcPreManifestLoop
	v2Server := v2.NewServer(workerServer, nil, nil, v2.ServerConfig{})
	require.NotNil(t, v2Server)
	t.Cleanup(v2Server.Shutdown)

	specs := []worker.BootcInfoResolveJobSpec{
		{
			Ref:         "quay.io/centos-bootc/centos-bootc:stream9",
			ResolveMode: worker.BootcInfoResolveModeFull,
		},
	}
	archi := arch.ARCH_X86_64.String()

	preManifestJobID := enqueuePreManifestWithResolvedDep(t, workerServer, archi, specs)

	// Wait for the loop to pick up and finish the pre-manifest job.
	// Poll with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			require.FailNow(t, "timed out waiting for bootcPreManifestLoop to finish job")
		default:
		}

		var readResult worker.BootcPreManifestJobResult
		jobInfo, err := workerServer.BootcPreManifestJobInfo(preManifestJobID, &readResult)
		if err == nil && !jobInfo.JobStatus.Finished.IsZero() {
			assertValidPreManifestResult(t, readResult, archi, specs)
			return
		}

		// Small sleep to avoid busy-waiting
		time.Sleep(50 * time.Millisecond)
	}
}

// TestSerializeManifest_PreManifestErrors tests that serializeManifest
// correctly handles pre-manifest job failures and build version mismatches.
func TestSerializeManifest_PreManifestErrors(t *testing.T) {
	tests := []struct {
		name              string
		preManifestResult worker.BootcPreManifestJobResult
		checkResult       func(t *testing.T, result worker.ManifestJobByIDResult)
	}{
		{
			name: "dependency_failed",
			preManifestResult: worker.BootcPreManifestJobResult{
				JobResult: worker.JobResult{
					JobError: clienterrors.New(
						clienterrors.ErrorManifestGeneration,
						"simulated pre-manifest failure", nil,
					),
				},
			},
			checkResult: func(t *testing.T, result worker.ManifestJobByIDResult) {
				require.NotNil(t, result.JobError, "expected job to fail but it succeeded")
				assert.Equal(t, clienterrors.ErrorJobDependency, result.JobError.ID)
				assert.Contains(t, result.JobError.Reason, "bootc pre-manifest job dependency")
				assert.Nil(t, result.JobError.Details)
			},
		},
		{
			name: "version_mismatch",
			preManifestResult: worker.BootcPreManifestJobResult{
				ManifestInfo: worker.ManifestInfo{
					OSBuildComposerVersion: "git-rev:FAKE_DIFFERENT_VERSION",
					OSBuildComposerDeps: []*worker.OSBuildComposerDepModule{
						{Path: "github.com/osbuild/images", Version: "v999.999.999"},
					},
				},
			},
			checkResult: func(t *testing.T, result worker.ManifestJobByIDResult) {
				require.NotNil(t, result.JobError, "expected job to fail but it succeeded")
				assert.Equal(t, clienterrors.ErrorBuildVersionMismatch, result.JobError.ID)
				assert.Contains(t, result.JobError.Reason, "different composer builds")
				assert.Equal(t, map[string]interface{}{
					"upstream_version": "git-rev:FAKE_DIFFERENT_VERSION",
					"local_version":    common.BuildVersion(),
					"upstream_deps": []interface{}{
						map[string]interface{}{
							"path":    "github.com/osbuild/images",
							"version": "v999.999.999",
						},
					},
					// local_deps is nil because debug.ReadBuildInfo() fails in test binaries.
					// See https://github.com/golang/go/issues/33976
					"local_deps": nil,
				}, result.JobError.Details)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workerServer := newTestWorkerServer(t)

			preManifestJob := &worker.BootcPreManifestJob{
				ImageType:                  "qcow2",
				Seed:                       42,
				BootcInfoResolveDynArgsIdx: common.ToPtr(0),
			}
			preManifestJobID, err := workerServer.EnqueueBootcPreManifestJob(preManifestJob, nil, "")
			require.NoError(t, err)

			jobID, token, _, _, _, err := workerServer.RequestJob(
				context.Background(), "",
				[]string{worker.JobTypeBootcPreManifest}, []string{""}, uuid.Nil,
			)
			require.NoError(t, err)
			require.Equal(t, preManifestJobID, jobID)

			preManifestResultRaw, err := json.Marshal(tt.preManifestResult)
			require.NoError(t, err)

			err = workerServer.FinishJob(token, preManifestResultRaw)
			require.NoError(t, err)

			manifestJobID, err := workerServer.EnqueueManifestJobByID(
				&worker.ManifestJobByID{},
				[]uuid.UUID{preManifestJobID},
				"",
			)
			require.NoError(t, err)

			dependencies := v2.NewManifestJobDependencies(
				uuid.Nil,         // depsolveJobID
				uuid.Nil,         // containerResolveJobID
				uuid.Nil,         // ostreeResolveJobID
				uuid.Nil,         // bootcInfoResolveJobID
				preManifestJobID, // bootcPreManifestJobID
			)

			getManifestSource := func() (*manifest.Manifest, error) {
				return nil, fmt.Errorf("should not be called — error should be caught first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			done := make(chan struct{})
			go func() {
				defer close(done)
				v2.SerializeManifest(ctx, getManifestSource, workerServer, dependencies, manifestJobID, 42)
			}()

			select {
			case <-done:
			case <-time.After(15 * time.Second):
				t.Fatal("timed out waiting for serializeManifest to finish")
			}

			var manifestResult worker.ManifestJobByIDResult
			jobInfo, err := workerServer.ManifestJobInfo(manifestJobID, &manifestResult)
			require.NoError(t, err)
			require.NotNil(t, jobInfo)
			assert.False(t, jobInfo.JobStatus.Finished.IsZero(), "manifest job should be finished")

			tt.checkResult(t, manifestResult)
		})
	}
}
