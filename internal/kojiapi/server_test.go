package kojiapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/distro/test_distro"
	"github.com/osbuild/osbuild-composer/internal/dnfjson"
	"github.com/osbuild/osbuild-composer/internal/kojiapi"
	"github.com/osbuild/osbuild-composer/internal/kojiapi/api"
	distro_mock "github.com/osbuild/osbuild-composer/internal/mocks/distro"
	rpmmd_mock "github.com/osbuild/osbuild-composer/internal/mocks/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/test"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

var dnfjsonPath string

func setupDNFJSON() {
	// compile the mock-dnf-json binary to speed up tests
	tmpdir, err := os.MkdirTemp("", "")
	if err != nil {
		panic(err)
	}
	dnfjsonPath = filepath.Join(tmpdir, "mock-dnf-json")
	cmd := exec.Command("go", "build", "-o", dnfjsonPath, "../../cmd/mock-dnf-json")
	if err := cmd.Run(); err != nil {
		panic(err)
	}
}

func newTestKojiServer(t *testing.T, dir string) (*kojiapi.Server, *worker.Server) {

	// create tempdir subdirectory for store
	dbpath, err := os.MkdirTemp(dir, "")
	if err != nil {
		panic(err)
	}
	rpm_fixture := rpmmd_mock.BaseFixture(dbpath)

	distros, err := distro_mock.NewDefaultRegistry()
	require.NoError(t, err)
	require.NotNil(t, distros)

	solver := dnfjson.NewBaseSolver("") // test solver doesn't need a cache dir
	// create tempdir subdirectory for solver response file
	dspath, err := os.MkdirTemp(dir, "")
	if err != nil {
		panic(err)
	}

	respfile := rpm_fixture.ResponseGenerator(dspath)
	solver.SetDNFJSONPath(dnfjsonPath, respfile)
	kojiServer := kojiapi.NewServer(nil, rpm_fixture.Workers, solver, distros)
	require.NotNil(t, kojiServer)

	return kojiServer, rpm_fixture.Workers
}

func TestStatus(t *testing.T) {
	kojiServer, _ := newTestKojiServer(t, t.TempDir())
	handler := kojiServer.Handler("/api/composer-koji/v1")
	test.TestRoute(t, handler, false, "GET", "/api/composer-koji/v1/status", ``, http.StatusOK, `{"status":"OK"}`, "message")
}

type jobResult struct {
	Result interface{} `json:"result"`
}

func TestCompose(t *testing.T) {
	kojiServer, workerServer := newTestKojiServer(t, t.TempDir())
	handler := kojiServer.Handler("/api/composer-koji/v1")
	workerHandler := workerServer.Handler()

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
			composeReply:     `{"koji_build_id":42}`,
			composeStatus: `{
				"image_statuses": [
					{
						"status": "success"
					},
					{
						"status": "success"
					}
				],
				"koji_build_id": 42,
				"koji_task_id": 0,
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
			composeReplyCode: http.StatusBadRequest,
			composeReply:     `{"message":"Could not initialize build with koji: failure"}`,
			composeStatus: `{
				"image_statuses": [
					{
						"status": "failure"
					},
					{
						"status": "failure"
					}
				],
				"koji_task_id": 0,
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
			composeReplyCode: http.StatusBadRequest,
			composeReply:     `{"message":"Could not initialize build with koji: Koji init error"}`,
			composeStatus: `{
				"image_statuses": [
					{
						"status": "failure"
					},
					{
						"status": "failure"
					}
				],
				"koji_task_id": 0,
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
			composeReply:     `{"koji_build_id":42}`,
			composeStatus: `{
				"image_statuses": [
					{
						"status": "failure"
					},
					{
						"status": "success"
					}
				],
				"koji_build_id": 42,
				"koji_task_id": 0,
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
			composeReply:     `{"koji_build_id":42}`,
			composeStatus: `{
				"image_statuses": [
					{
						"status": "failure"
					},
					{
						"status": "success"
					}
				],
				"koji_build_id": 42,
				"koji_task_id": 0,
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
			composeReply:     `{"koji_build_id":42}`,
			composeStatus: `{
				"image_statuses": [
					{
						"status": "failure"
					},
					{
						"status": "success"
					}
				],
				"koji_build_id": 42,
				"koji_task_id": 0,
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
			composeReply:     `{"koji_build_id":42}`,
			composeStatus: `{
				"image_statuses": [
					{
						"status": "success"
					},
					{
						"status": "success"
					}
				],
				"koji_build_id": 42,
				"koji_task_id": 0,
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
			composeReply:     `{"koji_build_id":42}`,
			composeStatus: `{
				"image_statuses": [
					{
						"status": "success"
					},
					{
						"status": "success"
					}
				],
				"koji_build_id": 42,
				"koji_task_id": 0,
				"status": "failure"
			}`,
		},
	}
	for _, c := range cases {
		var wg sync.WaitGroup
		wg.Add(1)

		go func(t *testing.T, result worker.KojiInitJobResult) {
			_, token, jobType, rawJob, _, err := workerServer.RequestJob(context.Background(), test_distro.TestArchName, []string{worker.JobTypeKojiInit}, []string{""})
			require.NoError(t, err)
			require.Equal(t, worker.JobTypeKojiInit, jobType)

			var initJob worker.KojiInitJob
			err = json.Unmarshal(rawJob, &initJob)
			require.NoError(t, err)
			require.Equal(t, "koji.example.com", initJob.Server)
			require.Equal(t, "foo", initJob.Name)
			require.Equal(t, "1", initJob.Version)
			require.Equal(t, "2", initJob.Release)

			initJobResult, err := json.Marshal(&jobResult{Result: result})
			require.NoError(t, err)
			test.TestRoute(t, workerHandler, false, "PATCH", fmt.Sprintf("/api/worker/v1/jobs/%v", token), string(initJobResult), http.StatusOK,
				fmt.Sprintf(`{"href":"/api/worker/v1/jobs/%v","id":"%v","kind":"UpdateJobResponse"}`, token, token))

			wg.Done()
		}(t, c.initResult)

		test.TestRoute(t, handler, false, "POST", "/api/composer-koji/v1/compose", fmt.Sprintf(`
		{
			"name":"foo",
			"version":"1",
			"release":"2",
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
				"server": "koji.example.com"
			}
		}`, test_distro.TestDistroName, test_distro.TestArchName, test_distro.TestImageTypeName),
			c.composeReplyCode, c.composeReply, "id")
		wg.Wait()

		_, token, jobType, rawJob, _, err := workerServer.RequestJob(context.Background(), test_distro.TestArchName, []string{worker.JobTypeOSBuildKoji}, []string{""})
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

		_, token, jobType, rawJob, _, err = workerServer.RequestJob(context.Background(), test_distro.TestArchName, []string{worker.JobTypeOSBuildKoji}, []string{""})
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
		}`, test_distro.TestArchName, test_distro.TestDistroName), http.StatusOK,
			fmt.Sprintf(`{"href":"/api/worker/v1/jobs/%v","id":"%v","kind":"UpdateJobResponse"}`, token, token))

		finalizeID, token, jobType, rawJob, _, err := workerServer.RequestJob(context.Background(), test_distro.TestArchName, []string{worker.JobTypeKojiFinalize}, []string{""})
		require.NoError(t, err)
		require.Equal(t, worker.JobTypeKojiFinalize, jobType)

		var kojiFinalizeJob worker.KojiFinalizeJob
		err = json.Unmarshal(rawJob, &kojiFinalizeJob)
		require.NoError(t, err)
		require.Equal(t, "koji.example.com", kojiFinalizeJob.Server)
		require.Equal(t, "1", kojiFinalizeJob.Version)
		require.Equal(t, "2", kojiFinalizeJob.Release)
		require.ElementsMatch(t, []string{
			fmt.Sprintf("foo-1-2.%s.img", test_distro.TestArchName),
			fmt.Sprintf("foo-1-2.%s.img", test_distro.TestArchName),
		}, kojiFinalizeJob.KojiFilenames)
		require.NotEmpty(t, kojiFinalizeJob.KojiDirectory)

		finalizeResult, err := json.Marshal(&jobResult{Result: c.finalizeResult})
		require.NoError(t, err)
		test.TestRoute(t, workerHandler, false, "PATCH", fmt.Sprintf("/api/worker/v1/jobs/%v", token), string(finalizeResult), http.StatusOK,
			fmt.Sprintf(`{"href":"/api/worker/v1/jobs/%v","id":"%v","kind":"UpdateJobResponse"}`, token, token))

		test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/composer-koji/v1/compose/%v", finalizeID), ``, http.StatusOK, c.composeStatus)

		test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/composer-koji/v1/compose/%v/manifests", finalizeID), ``, http.StatusOK, `[{"version": "", "pipelines": [], "sources": {}}, {"version": "", "pipelines": [], "sources": {}}]`)
	}
}

func TestRequest(t *testing.T) {
	server, _ := newTestKojiServer(t, t.TempDir())
	handler := server.Handler("/api/composer-koji/v1")

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
	req = httptest.NewRequest("GET", "/api/composer-koji/v1/compose/badid", nil)

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	resp = rec.Result()

	err = json.NewDecoder(resp.Body).Decode(&status)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestJobTypeValidation(t *testing.T) {
	dir := t.TempDir()

	server, workers := newTestKojiServer(t, dir)
	handler := server.Handler("/api/composer-koji/v1")

	// Enqueue a compose job with N images (+ an Init and a Finalize job)
	// Enqueuing them manually gives us access to the job IDs to use in
	// requests.
	nImages := 4
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
		test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/composer-koji/v1/compose/%s%s", finalizeID, path), ``, http.StatusOK, "*")

		// The other IDs should fail
		msg := fmt.Sprintf("Job %s not found: expected \"koji-finalize\", found \"koji-init\" job instead", initID)
		resp, _ := json.Marshal(map[string]string{"message": msg})
		test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/composer-koji/v1/compose/%s%s", initID, path), ``, http.StatusNotFound, string(resp))

		for idx, buildID := range buildJobIDs {
			msg := fmt.Sprintf("Job %s not found: expected \"koji-finalize\", found \"osbuild-koji:fake-arch-%d\" job instead", buildID, idx)
			resp, _ := json.Marshal(map[string]string{"message": msg})
			test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/composer-koji/v1/compose/%s%s", buildID, path), ``, http.StatusNotFound, string(resp))
		}

		badID := uuid.New()
		msg = fmt.Sprintf("Job %s not found: job does not exist", badID)
		resp, _ = json.Marshal(map[string]string{"message": msg})
		test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/composer-koji/v1/compose/%s%s", badID, path), ``, http.StatusNotFound, string(resp))
	}
}

func TestMain(m *testing.M) {
	setupDNFJSON()
	defer os.RemoveAll(dnfjsonPath)
	code := m.Run()
	os.Exit(code)
}
