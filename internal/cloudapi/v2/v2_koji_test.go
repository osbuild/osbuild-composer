package v2_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	v2 "github.com/osbuild/osbuild-composer/internal/cloudapi/v2"
	"github.com/osbuild/osbuild-composer/internal/distro/test_distro"
	"github.com/osbuild/osbuild-composer/internal/kojiapi/api"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/test"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type jobResult struct {
	Result interface{} `json:"result"`
}

func TestKojiCompose(t *testing.T) {
	kojiServer, workerServer, _, cancel := newV2Server(t, t.TempDir(), []string{""}, false)
	handler := kojiServer.Handler("/api/image-builder-composer/v2")
	workerHandler := workerServer.Handler()
	defer cancel()

	type kojiCase struct {
		initResult       worker.KojiInitJobResult
		buildResult      worker.OSBuildKojiJobResult
		finalizeResult   worker.KojiFinalizeJobResult
		composeReplyCode int
		composeReply     string
		composeStatus    string
	}

	var cases = []kojiCase{
		{
			initResult: worker.KojiInitJobResult{
				BuildID: 42,
				Token:   `"foobar"`,
			},
			buildResult: worker.OSBuildKojiJobResult{
				Arch:      test_distro.TestArchName,
				HostOS:    test_distro.TestDistroName,
				ImageHash: "browns",
				ImageSize: 42,
				OSBuildOutput: &osbuild.Result{
					Success: true,
				},
			},
			composeReplyCode: http.StatusCreated,
			composeReply:     `{"href":"/api/image-builder-composer/v2/compose", "kind":"ComposeId"}`,
			composeStatus: `{
				"kind": "ComposeStatus",
				"image_status": {
					"status": "success"
				},
				"image_statuses": [
					{
						"status": "success"
					},
					{
						"status": "success"
					}
				],
				"koji_status": {
					"build_id": 42
				},
				"status": "success"
			}`,
		},
		{
			initResult: worker.KojiInitJobResult{
				KojiError: "failure",
			},
			buildResult: worker.OSBuildKojiJobResult{
				Arch:      test_distro.TestArchName,
				HostOS:    test_distro.TestDistroName,
				ImageHash: "browns",
				ImageSize: 42,
				OSBuildOutput: &osbuild.Result{
					Success: true,
				},
			},
			composeReplyCode: http.StatusCreated,
			composeReply:     `{"href":"/api/image-builder-composer/v2/compose", "kind":"ComposeId"}`,
			composeStatus: `{
				"kind": "ComposeStatus",
				"image_status": {
					"status": "failure"
				},
				"image_statuses": [
					{
						"status": "failure"
					},
					{
						"status": "failure"
					}
				],
				"koji_status": {},
				"status": "failure"
			}`,
		},
		{
			initResult: worker.KojiInitJobResult{
				JobResult: worker.JobResult{
					JobError: clienterrors.WorkerClientError(clienterrors.ErrorKojiInit, "Koji init error"),
				},
			},
			buildResult: worker.OSBuildKojiJobResult{
				Arch:      test_distro.TestArchName,
				HostOS:    test_distro.TestDistroName,
				ImageHash: "browns",
				ImageSize: 42,
				OSBuildOutput: &osbuild.Result{
					Success: true,
				},
			},
			composeReplyCode: http.StatusCreated,
			composeReply:     `{"href":"/api/image-builder-composer/v2/compose", "kind":"ComposeId"}`,
			composeStatus: `{
				"kind": "ComposeStatus",
				"image_status": {
					"status": "failure"
				},
				"image_statuses": [
					{
						"status": "failure"
					},
					{
						"status": "failure"
					}
				],
				"koji_status": {},
				"status": "failure"
			}`,
		},
		{
			initResult: worker.KojiInitJobResult{
				BuildID: 42,
				Token:   `"foobar"`,
			},
			buildResult: worker.OSBuildKojiJobResult{
				Arch:      test_distro.TestArchName,
				HostOS:    test_distro.TestDistroName,
				ImageHash: "browns",
				ImageSize: 42,
				OSBuildOutput: &osbuild.Result{
					Success: false,
				},
			},
			composeReplyCode: http.StatusCreated,
			composeReply:     `"href":"/api/image-builder-composer/v2/compose", "kind":"ComposeId"`,
			composeStatus: `{
				"kind": "ComposeStatus",
				"image_status": {
					"status": "failure"
				},
				"image_statuses": [
					{
						"status": "failure"
					},
					{
						"status": "success"
					}
				],
				"koji_status": {
					"build_id": 42
				},
				"status": "failure"
			}`,
		},
		{
			initResult: worker.KojiInitJobResult{
				BuildID: 42,
				Token:   `"foobar"`,
			},
			buildResult: worker.OSBuildKojiJobResult{
				Arch:      test_distro.TestArchName,
				HostOS:    test_distro.TestDistroName,
				ImageHash: "browns",
				ImageSize: 42,
				OSBuildOutput: &osbuild.Result{
					Success: true,
				},
				KojiError: "failure",
			},
			composeReplyCode: http.StatusCreated,
			composeReply:     `"href":"/api/image-builder-composer/v2/compose", "kind":"ComposeId"`,
			composeStatus: `{
				"kind": "ComposeStatus",
				"image_status": {
					"status": "failure"
				},
				"image_statuses": [
					{
						"status": "failure"
					},
					{
						"status": "success"
					}
				],
				"koji_status": {
					"build_id": 42
				},
				"status": "failure"
			}`,
		},
		{
			initResult: worker.KojiInitJobResult{
				BuildID: 42,
				Token:   `"foobar"`,
			},
			buildResult: worker.OSBuildKojiJobResult{
				Arch:      test_distro.TestArchName,
				HostOS:    test_distro.TestDistroName,
				ImageHash: "browns",
				ImageSize: 42,
				OSBuildOutput: &osbuild.Result{
					Success: true,
				},
				JobResult: worker.JobResult{
					JobError: clienterrors.WorkerClientError(clienterrors.ErrorBuildJob, "Koji build error"),
				},
			},
			composeReplyCode: http.StatusCreated,
			composeReply:     `"href":"/api/image-builder-composer/v2/compose", "kind":"ComposeId"`,
			composeStatus: `{
				"kind": "ComposeStatus",
				"image_status": {
					"status": "failure"
				},
				"image_statuses": [
					{
						"status": "failure"
					},
					{
						"status": "success"
					}
				],
				"koji_status": {
					"build_id": 42
				},
				"status": "failure"
			}`,
		},
		{
			initResult: worker.KojiInitJobResult{
				BuildID: 42,
				Token:   `"foobar"`,
			},
			buildResult: worker.OSBuildKojiJobResult{
				Arch:      test_distro.TestArchName,
				HostOS:    test_distro.TestDistroName,
				ImageHash: "browns",
				ImageSize: 42,
				OSBuildOutput: &osbuild.Result{
					Success: true,
				},
			},
			finalizeResult: worker.KojiFinalizeJobResult{
				KojiError: "failure",
			},
			composeReplyCode: http.StatusCreated,
			composeReply:     `"href":"/api/image-builder-composer/v2/compose", "kind":"ComposeId"`,
			composeStatus: `{
				"kind": "ComposeStatus",
				"image_status": {
					"status": "success"
				},
				"image_statuses": [
					{
						"status": "success"
					},
					{
						"status": "success"
					}
				],
				"koji_status": {
					"build_id": 42
				},
				"status": "failure"
			}`,
		},
		{
			initResult: worker.KojiInitJobResult{
				BuildID: 42,
				Token:   `"foobar"`,
			},
			buildResult: worker.OSBuildKojiJobResult{
				Arch:      test_distro.TestArchName,
				HostOS:    test_distro.TestDistroName,
				ImageHash: "browns",
				ImageSize: 42,
				OSBuildOutput: &osbuild.Result{
					Success: true,
				},
			},
			finalizeResult: worker.KojiFinalizeJobResult{
				JobResult: worker.JobResult{
					JobError: clienterrors.WorkerClientError(clienterrors.ErrorKojiFinalize, "Koji finalize error"),
				},
			},
			composeReplyCode: http.StatusCreated,
			composeReply:     `"href":"/api/image-builder-composer/v2/compose", "kind":"ComposeId"`,
			composeStatus: `{
				"kind": "ComposeStatus",
				"image_status": {
					"status": "success"
				},
				"image_statuses": [
					{
						"status": "success"
					},
					{
						"status": "success"
					}
				],
				"koji_status": {
					"build_id": 42
				},
				"status": "failure"
			}`,
		},
		{
			initResult: worker.KojiInitJobResult{
				BuildID: 42,
				Token:   `"foobar"`,
			},
			buildResult: worker.OSBuildKojiJobResult{
				Arch:      test_distro.TestArchName,
				HostOS:    test_distro.TestDistroName,
				ImageHash: "browns",
				ImageSize: 42,
				OSBuildOutput: &osbuild.Result{
					Success: true,
				},
				JobResult: worker.JobResult{
					JobError: clienterrors.WorkerClientError(
						clienterrors.ErrorManifestDependency,
						"Manifest dependency failed",
						clienterrors.WorkerClientError(
							clienterrors.ErrorDNFOtherError,
							"DNF Error",
						),
					),
				},
			},
			composeReplyCode: http.StatusCreated,
			composeReply:     `"href":"/api/image-builder-composer/v2/compose", "kind":"ComposeId"`,
			composeStatus: `{
				"kind": "ComposeStatus",
				"image_status": {
					"error": {
						"details": {
							"id": 22,
							"reason": "DNF Error"
						},
						"id": 9,
						"reason": "Manifest dependency failed"
					},
					"status": "failure",
				},
				"image_statuses": [
					{
						"error": {
							"details": {
								"id": 22,
								"reason": "Error in depsolve job"
							},
							"id": 9,
							"reason": "Manifest dependency failed"
						},
						"status": "failure"
					},
					{
						"status": "success"
					}
				],
				"koji_status": {
					"build_id": 42
				},
				"status": "failure"
			}`,
		},
	}
	for _, c := range cases[2:3] {
		test.TestRoute(t, handler, false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
		{
			"distribution":"%[1]s",
			"image_requests": [
				{
					"architecture": "%[2]s",
					"image_type": "%[3]s",
					"repositories": [
						{
							"baseurl": "https://repo.example.com/"
						}
					]
				},
				{
					"architecture": "%[2]s",
					"image_type": "%[3]s",
					"repositories": [
						{
							"baseurl": "https://repo.example.com/"
						}
					]
				}
			],
			"koji": {
				"server": "koji.example.com",
				"name":"foo",
				"version":"1",
				"release":"2",
				"task_id": 42
			}
		}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypesGuestImage)),
			c.composeReplyCode, c.composeReply, "id", "operation_id")

		// handle koji-init
		_, token, jobType, rawJob, _, err := workerServer.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeKojiInit}, []string{""})
		require.NoError(t, err)
		require.Equal(t, worker.JobTypeKojiInit, jobType)

		var initJob worker.KojiInitJob
		err = json.Unmarshal(rawJob, &initJob)
		require.NoError(t, err)
		require.Equal(t, "koji.example.com", initJob.Server)
		require.Equal(t, "foo", initJob.Name)
		require.Equal(t, "1", initJob.Version)
		require.Equal(t, "2", initJob.Release)

		initJobResult, err := json.Marshal(&jobResult{Result: c.initResult})
		require.NoError(t, err)
		test.TestRoute(t, workerHandler, false, "PATCH", fmt.Sprintf("/api/worker/v1/jobs/%v", token), string(initJobResult), http.StatusOK,
			fmt.Sprintf(`{"href":"/api/worker/v1/jobs/%v","id":"%v","kind":"UpdateJobResponse"}`, token, token))

		// handle osbuild-koji #1
		_, token, jobType, rawJob, _, err = workerServer.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeOSBuildKoji}, []string{""})
		require.NoError(t, err)
		require.Equal(t, worker.JobTypeOSBuildKoji, jobType)

		var osbuildJob worker.OSBuildKojiJob
		err = json.Unmarshal(rawJob, &osbuildJob)
		require.NoError(t, err)
		require.Equal(t, "koji.example.com", osbuildJob.KojiServer)
		require.Equal(t, "test.img", osbuildJob.ImageName)
		require.NotEmpty(t, osbuildJob.KojiDirectory)

		buildJobResult, err := json.Marshal(&jobResult{Result: c.buildResult})
		require.NoError(t, err)
		test.TestRoute(t, workerHandler, false, "PATCH", fmt.Sprintf("/api/worker/v1/jobs/%v", token), string(buildJobResult), http.StatusOK,
			fmt.Sprintf(`{"href":"/api/worker/v1/jobs/%v","id":"%v","kind":"UpdateJobResponse"}`, token, token))

		// handle osbuild-koji #2
		_, token, jobType, rawJob, _, err = workerServer.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeOSBuildKoji}, []string{""})
		require.NoError(t, err)
		require.Equal(t, worker.JobTypeOSBuildKoji, jobType)

		err = json.Unmarshal(rawJob, &osbuildJob)
		require.NoError(t, err)
		require.Equal(t, "koji.example.com", osbuildJob.KojiServer)
		require.Equal(t, "test.img", osbuildJob.ImageName)
		require.NotEmpty(t, osbuildJob.KojiDirectory)

		test.TestRoute(t, workerHandler, false, "PATCH", fmt.Sprintf("/api/worker/v1/jobs/%v", token), fmt.Sprintf(`{
				"result": {
					"arch": "%s",
					"host_os": "%s",
					"image_hash": "browns",
					"image_size": 42,
					"osbuild_output": {
						"success": true
					}
				}
			}`, test_distro.TestArch3Name, test_distro.TestDistroName), http.StatusOK,
			fmt.Sprintf(`{"href":"/api/worker/v1/jobs/%v","id":"%v","kind":"UpdateJobResponse"}`, token, token))

		// handle koji-finalize
		finalizeID, token, jobType, rawJob, _, err := workerServer.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeKojiFinalize}, []string{""})
		require.NoError(t, err)
		require.Equal(t, worker.JobTypeKojiFinalize, jobType)

		var kojiFinalizeJob worker.KojiFinalizeJob
		err = json.Unmarshal(rawJob, &kojiFinalizeJob)
		require.NoError(t, err)
		require.Equal(t, "koji.example.com", kojiFinalizeJob.Server)
		require.Equal(t, "1", kojiFinalizeJob.Version)
		require.Equal(t, "2", kojiFinalizeJob.Release)
		require.ElementsMatch(t, []string{
			fmt.Sprintf("foo-1-2.%s.img", test_distro.TestArch3Name),
			fmt.Sprintf("foo-1-2.%s.img", test_distro.TestArch3Name),
		}, kojiFinalizeJob.KojiFilenames)
		require.NotEmpty(t, kojiFinalizeJob.KojiDirectory)

		finalizeResult, err := json.Marshal(&jobResult{Result: c.finalizeResult})
		require.NoError(t, err)
		test.TestRoute(t, workerHandler, false, "PATCH", fmt.Sprintf("/api/worker/v1/jobs/%v", token), string(finalizeResult), http.StatusOK,
			fmt.Sprintf(`{"href":"/api/worker/v1/jobs/%v","id":"%v","kind":"UpdateJobResponse"}`, token, token))

		// get the status
		test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", finalizeID), ``, http.StatusOK, c.composeStatus, `href`, `id`)

		// get the manifests
		test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/manifests", finalizeID), ``, http.StatusOK, `{"manifests":[null,null],"kind":"ComposeManifests"}`, `href`, `id`)

		// get the logs
		test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/logs", finalizeID), ``, http.StatusOK, `{"kind":"ComposeLogs"}`, `koji`, `image_builds`, `href`, `id`)
	}
}

func TestKojiRequest(t *testing.T) {
	server, _, _, cancel := newV2Server(t, t.TempDir(), []string{""}, false)
	handler := server.Handler("/api/image-builder-composer/v2")
	defer cancel()

	// Make request to an invalid route
	req := httptest.NewRequest("GET", "/invalidroute", nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	resp := rec.Result()

	var status api.Status
	err := json.NewDecoder(resp.Body).Decode(&status)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Trigger an error 400 code
	req = httptest.NewRequest("GET", "/api/image-builder-composer/v2/composes/badid", nil)

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	resp = rec.Result()

	err = json.NewDecoder(resp.Body).Decode(&status)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestKojiJobTypeValidation(t *testing.T) {
	server, workers, _, cancel := newV2Server(t, t.TempDir(), []string{""}, false)
	handler := server.Handler("/api/image-builder-composer/v2")
	defer cancel()

	// Enqueue a compose job with N images (+ an Init and a Finalize job)
	// Enqueuing them manually gives us access to the job IDs to use in
	// requests.
	// TODO: set to 4
	nImages := 1
	initJob := worker.KojiInitJob{
		Server:  "test-server",
		Name:    "test-job",
		Version: "42",
		Release: "1",
	}
	initID, err := workers.EnqueueKojiInit(&initJob, "")
	require.NoError(t, err)

	buildJobs := make([]worker.OSBuildKojiJob, nImages)
	buildJobIDs := make([]uuid.UUID, nImages)
	filenames := make([]string, nImages)
	for idx := 0; idx < nImages; idx++ {
		fname := fmt.Sprintf("image-file-%04d", idx)
		buildJob := worker.OSBuildKojiJob{
			ImageName:     fmt.Sprintf("build-job-%04d", idx),
			KojiServer:    "test-server",
			KojiDirectory: "koji-server-test-dir",
			KojiFilename:  fname,
		}
		buildID, err := workers.EnqueueOSBuildKoji(fmt.Sprintf("fake-arch-%d", idx), &buildJob, initID, "")
		require.NoError(t, err)

		buildJobs[idx] = buildJob
		buildJobIDs[idx] = buildID
		filenames[idx] = fname
	}

	finalizeJob := worker.KojiFinalizeJob{
		Server:        "test-server",
		Name:          "test-job",
		Version:       "42",
		Release:       "1",
		KojiFilenames: filenames,
		KojiDirectory: "koji-server-test-dir",
		TaskID:        0,
		StartTime:     uint64(time.Now().Unix()),
	}
	finalizeID, err := workers.EnqueueKojiFinalize(&finalizeJob, initID, buildJobIDs, "")
	require.NoError(t, err)

	// ----- Jobs queued - Test API endpoints (status, manifests, logs) ----- //

	for _, path := range []string{"", "/manifests", "/logs"} {
		// should return OK - actual result should be tested elsewhere
		test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%s%s", finalizeID, path), ``, http.StatusOK, "*")

		// The other IDs should fail
		test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%s%s", initID, path), ``, http.StatusNotFound, `{"code":"IMAGE-BUILDER-COMPOSER-26","href":"/api/image-builder-composer/v2/errors/26","id":"26","kind":"Error","reason":"Requested job has invalid type"}`, `operation_id`)

		for _, buildID := range buildJobIDs {
			test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%s%s", buildID, path), ``, http.StatusNotFound, `{"code":"IMAGE-BUILDER-COMPOSER-26","href":"/api/image-builder-composer/v2/errors/26","id":"26","kind":"Error","reason":"Requested job has invalid type"}`, `operation_id`)
		}

		badID := uuid.New()
		test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%s%s", badID, path), ``, http.StatusNotFound, `{"code":"IMAGE-BUILDER-COMPOSER-15","href":"/api/image-builder-composer/v2/errors/15","id":"15","kind":"Error","reason":"Compose with given id not found"}`, `operation_id`)
	}
}
