package v2_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	v2 "github.com/osbuild/osbuild-composer/internal/cloudapi/v2"
	"github.com/osbuild/osbuild-composer/internal/distro/test_distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/test"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type jobResult struct {
	Result interface{} `json:"result"`
}

func TestKojiCompose(t *testing.T) {
	kojiServer, workerServer, _, cancel := newV2Server(t, t.TempDir(), []string{""}, false, false)
	handler := kojiServer.Handler("/api/image-builder-composer/v2")
	workerHandler := workerServer.Handler()
	defer cancel()

	type kojiCase struct {
		initResult       worker.KojiInitJobResult
		buildResult      worker.OSBuildJobResult
		finalizeResult   worker.KojiFinalizeJobResult
		composeReplyCode int
		composeReply     string
		composeStatus    string
	}

	var cases = []kojiCase{
		// #0
		{
			initResult: worker.KojiInitJobResult{
				BuildID: 42,
				Token:   `"foobar"`,
			},
			buildResult: worker.OSBuildJobResult{
				Arch:   test_distro.TestArchName,
				HostOS: test_distro.TestDistroName,
				TargetResults: []*target.TargetResult{target.NewKojiTargetResult(&target.KojiTargetResultOptions{
					ImageMD5:  "browns",
					ImageSize: 42,
				})},
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
		// #1
		{
			initResult: worker.KojiInitJobResult{
				KojiError: "failure",
			},
			buildResult: worker.OSBuildJobResult{
				Arch:   test_distro.TestArchName,
				HostOS: test_distro.TestDistroName,
				TargetResults: []*target.TargetResult{target.NewKojiTargetResult(&target.KojiTargetResultOptions{
					ImageMD5:  "browns",
					ImageSize: 42,
				})},
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
		// #2
		{
			initResult: worker.KojiInitJobResult{
				JobResult: worker.JobResult{
					JobError: clienterrors.WorkerClientError(clienterrors.ErrorKojiInit, "Koji init error", nil),
				},
			},
			buildResult: worker.OSBuildJobResult{
				Arch:   test_distro.TestArchName,
				HostOS: test_distro.TestDistroName,
				TargetResults: []*target.TargetResult{target.NewKojiTargetResult(&target.KojiTargetResultOptions{
					ImageMD5:  "browns",
					ImageSize: 42,
				})},
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
		// #3
		{
			initResult: worker.KojiInitJobResult{
				BuildID: 42,
				Token:   `"foobar"`,
			},
			buildResult: worker.OSBuildJobResult{
				Arch:   test_distro.TestArchName,
				HostOS: test_distro.TestDistroName,
				TargetResults: []*target.TargetResult{target.NewKojiTargetResult(&target.KojiTargetResultOptions{
					ImageMD5:  "browns",
					ImageSize: 42,
				})},
				OSBuildOutput: &osbuild.Result{
					Success: false,
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
						"status": "success"
					}
				],
				"koji_status": {
					"build_id": 42
				},
				"status": "failure"
			}`,
		},
		// #4
		{
			initResult: worker.KojiInitJobResult{
				BuildID: 42,
				Token:   `"foobar"`,
			},
			buildResult: worker.OSBuildJobResult{
				Arch:   test_distro.TestArchName,
				HostOS: test_distro.TestDistroName,
				TargetResults: []*target.TargetResult{target.NewKojiTargetResult(&target.KojiTargetResultOptions{
					ImageMD5:  "browns",
					ImageSize: 42,
				})},
				OSBuildOutput: &osbuild.Result{
					Success: true,
				},
				JobResult: worker.JobResult{
					JobError: clienterrors.WorkerClientError(clienterrors.ErrorBuildJob, "Koji build error", nil),
				},
			},
			composeReplyCode: http.StatusCreated,
			composeReply:     `{"href":"/api/image-builder-composer/v2/compose", "kind":"ComposeId"}`,
			composeStatus: `{
				"kind": "ComposeStatus",
				"image_status": {
					"status": "failure",
					"error": {
						"id": 10,
						"reason": "Koji build error"
					}
				},
				"image_statuses": [
					{
						"status": "failure",
						"error": {
							"id": 10,
							"reason": "Koji build error"
						}
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
		// #5
		{
			initResult: worker.KojiInitJobResult{
				BuildID: 42,
				Token:   `"foobar"`,
			},
			buildResult: worker.OSBuildJobResult{
				Arch:   test_distro.TestArchName,
				HostOS: test_distro.TestDistroName,
				TargetResults: []*target.TargetResult{target.NewKojiTargetResult(&target.KojiTargetResultOptions{
					ImageMD5:  "browns",
					ImageSize: 42,
				})},
				OSBuildOutput: &osbuild.Result{
					Success: true,
				},
			},
			finalizeResult: worker.KojiFinalizeJobResult{
				KojiError: "failure",
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
				"status": "failure"
			}`,
		},
		// #6
		{
			initResult: worker.KojiInitJobResult{
				BuildID: 42,
				Token:   `"foobar"`,
			},
			buildResult: worker.OSBuildJobResult{
				Arch:   test_distro.TestArchName,
				HostOS: test_distro.TestDistroName,
				TargetResults: []*target.TargetResult{target.NewKojiTargetResult(&target.KojiTargetResultOptions{
					ImageMD5:  "browns",
					ImageSize: 42,
				})},
				OSBuildOutput: &osbuild.Result{
					Success: true,
				},
			},
			finalizeResult: worker.KojiFinalizeJobResult{
				JobResult: worker.JobResult{
					JobError: clienterrors.WorkerClientError(clienterrors.ErrorKojiFinalize, "Koji finalize error", nil),
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
				"status": "failure"
			}`,
		},
	}

	for idx, c := range cases {
		name, version, release := "foo", "1", "2"
		t.Run(fmt.Sprintf("Test case #%d", idx), func(t *testing.T) {
			composeRawReply := test.TestRouteWithReply(t, handler, false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
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
				"name":"%[4]s",
				"version":"%[5]s",
				"release":"%[6]s",
				"task_id": 42
			}
		}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypesGuestImage), name, version, release),
				c.composeReplyCode, c.composeReply, "id", "operation_id")

			// determine the compose ID from the reply
			var composeReply v2.ComposeId
			err := json.Unmarshal(composeRawReply, &composeReply)
			require.NoError(t, err)
			composeId, err := uuid.Parse(composeReply.Id)
			require.NoError(t, err)

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

			// Finishing of the goroutine handling the manifest job is not deterministic and as a result, we may get
			// the second osbuild job first.
			// The build jobs ID is determined from the dependencies of the koji-finalize job dependencies.
			finalizeInfo, err := workerServer.KojiFinalizeJobInfo(composeId, &worker.KojiFinalizeJobResult{})
			require.NoError(t, err)
			buildJobIDs := finalizeInfo.Deps[1:]
			require.Len(t, buildJobIDs, 2)

			// handle build jobs
			for i := 0; i < len(buildJobIDs); i++ {
				jobID, token, jobType, rawJob, _, err := workerServer.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeOSBuild}, []string{""})
				require.NoError(t, err)
				require.Equal(t, worker.JobTypeOSBuild, jobType)

				var osbuildJob worker.OSBuildJob
				err = json.Unmarshal(rawJob, &osbuildJob)
				require.NoError(t, err)
				jobTarget := osbuildJob.Targets[0].Options.(*target.KojiTargetOptions)
				require.Equal(t, "koji.example.com", jobTarget.Server)
				require.Equal(t, "test.img", osbuildJob.Targets[0].OsbuildArtifact.ExportFilename)
				require.Equal(t, fmt.Sprintf("%s-%s-%s.%s.img", name, version, release, test_distro.TestArch3Name),
					osbuildJob.Targets[0].ImageName)
				require.NotEmpty(t, jobTarget.UploadDirectory)

				var buildJobResult string
				switch jobID {
				// use the build job result from the test case only for the first job
				case buildJobIDs[0]:
					buildJobResultBytes, err := json.Marshal(&jobResult{Result: c.buildResult})
					require.NoError(t, err)
					buildJobResult = string(buildJobResultBytes)
				default:
					buildJobResult = fmt.Sprintf(`{
						"result": {
							"arch": "%s",
							"host_os": "%s",
							"image_hash": "browns",
							"image_size": 42,
							"osbuild_output": {
								"success": true
							}
						}
					}`, test_distro.TestArch3Name, test_distro.TestDistroName)
				}

				test.TestRoute(t, workerHandler, false, "PATCH", fmt.Sprintf("/api/worker/v1/jobs/%v", token), buildJobResult, http.StatusOK,
					fmt.Sprintf(`{"href":"/api/worker/v1/jobs/%v","id":"%v","kind":"UpdateJobResponse"}`, token, token))
			}

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
			test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/manifests", finalizeID), ``, http.StatusOK, `{"manifests":[{"version":"2","pipelines":[],"sources":{}},{"version":"2","pipelines":[],"sources":{}}],"kind":"ComposeManifests"}`, `href`, `id`)

			// get the logs
			test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/logs", finalizeID), ``, http.StatusOK, `{"kind":"ComposeLogs"}`, `koji`, `image_builds`, `href`, `id`)
		})
	}
}

func TestKojiJobTypeValidation(t *testing.T) {
	server, workers, _, cancel := newV2Server(t, t.TempDir(), []string{""}, false, false)
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

	manifest, err := json.Marshal(osbuild.Manifest{})
	require.NoErrorf(t, err, "error marshalling empty Manifest to JSON")

	buildJobs := make([]worker.OSBuildJob, nImages)
	buildJobIDs := make([]uuid.UUID, nImages)
	filenames := make([]string, nImages)
	for idx := 0; idx < nImages; idx++ {
		kojiTarget := target.NewKojiTarget(&target.KojiTargetOptions{
			Server:          "test-server",
			UploadDirectory: "koji-server-test-dir",
		})
		kojiTarget.OsbuildArtifact.ExportFilename = "test.img"
		kojiTarget.ImageName = fmt.Sprintf("image-file-%04d", idx)
		buildJob := worker.OSBuildJob{
			Targets: []*target.Target{kojiTarget},
			// Add an empty manifest as a static job argument to make the test pass.
			// Becasue of a bug in the API, the test was passing even without
			// any manifest being attached to the job (static or dynamic).
			// In reality, cloudapi never adds the manifest as a static job argument.
			// TODO: use dependent depsolve and manifests jobs instead
			Manifest: manifest,
		}
		buildID, err := workers.EnqueueOSBuildAsDependency(fmt.Sprintf("fake-arch-%d", idx), &buildJob, []uuid.UUID{initID}, "")
		require.NoError(t, err)

		buildJobs[idx] = buildJob
		buildJobIDs[idx] = buildID
		filenames[idx] = kojiTarget.OsbuildArtifact.ExportFilename
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

	t.Logf("%q job ID: %s", worker.JobTypeKojiInit, initID)
	t.Logf("%q job ID: %s", worker.JobTypeKojiFinalize, finalizeID)
	t.Logf("%q job IDs: %v", worker.JobTypeOSBuild, buildJobIDs)
	for _, path := range []string{"", "/manifests", "/logs"} {
		// should return OK - actual result should be tested elsewhere
		test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%s%s", finalizeID, path), ``, http.StatusOK, "*")

		// The other IDs should fail
		test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%s%s", initID, path), ``, http.StatusNotFound, `{"code":"IMAGE-BUILDER-COMPOSER-26", "details": "", "href":"/api/image-builder-composer/v2/errors/26","id":"26","kind":"Error","reason":"Job with given id has an invalid type"}`, `operation_id`)

		for _, buildID := range buildJobIDs {
			test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%s%s", buildID, path), ``, http.StatusOK, "*")
		}

		badID := uuid.New()
		test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%s%s", badID, path), ``, http.StatusNotFound, `{"code":"IMAGE-BUILDER-COMPOSER-15", "details": "", "href":"/api/image-builder-composer/v2/errors/15","id":"15","kind":"Error","reason":"Compose with given id not found"}`, `operation_id`)
	}
}
