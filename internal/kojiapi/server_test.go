package kojiapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	"github.com/osbuild/osbuild-composer/internal/kojiapi"
	"github.com/osbuild/osbuild-composer/internal/kojiapi/api"
	distro_mock "github.com/osbuild/osbuild-composer/internal/mocks/distro"
	rpmmd_mock "github.com/osbuild/osbuild-composer/internal/mocks/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/test"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/stretchr/testify/require"
)

func newTestKojiServer(t *testing.T, dir string) (*kojiapi.Server, *worker.Server) {
	rpm_fixture := rpmmd_mock.BaseFixture()
	rpm := rpmmd_mock.NewRPMMDMock(rpm_fixture)
	require.NotNil(t, rpm)

	distros, err := distro_mock.NewDefaultRegistry()
	require.NoError(t, err)
	require.NotNil(t, distros)

	queue, err := fsjobqueue.New(dir)
	require.NoError(t, err)

	workerServer := worker.NewServer(nil, queue, "")
	require.NotNil(t, workerServer)

	kojiServer := kojiapi.NewServer(nil, workerServer, rpm, distros)
	require.NotNil(t, kojiServer)

	return kojiServer, workerServer
}

func TestStatus(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-kojiapi-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	kojiServer, _ := newTestKojiServer(t, dir)
	handler := kojiServer.Handler("/api/composer-koji/v1")
	test.TestRoute(t, handler, false, "GET", "/api/composer-koji/v1/status", ``, http.StatusOK, `{"status":"OK"}`, "message")
}

func TestCompose(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-kojiapi-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	kojiServer, workerServer := newTestKojiServer(t, dir)
	handler := kojiServer.Handler("/api/composer-koji/v1")
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		token, _, jobType, rawJob, _, err := workerServer.RequestJob(context.Background(), "x86_64", []string{"koji-init"})
		require.NoError(t, err)
		require.Equal(t, "koji-init", jobType)

		var initJob worker.KojiInitJob
		err = json.Unmarshal(rawJob, &initJob)
		require.NoError(t, err)
		require.Equal(t, "koji.example.com", initJob.Server)
		require.Equal(t, "foo", initJob.Name)
		require.Equal(t, "1", initJob.Version)
		require.Equal(t, "2", initJob.Release)

		test.TestRoute(t, workerServer, false, "PATCH", fmt.Sprintf("/api/worker/v1/jobs/%v", token), `{
		"result": {
			"build_id": 42,
			"token": "foobar"
		}
	}`, http.StatusOK, `{}`)
		wg.Done()
	}()

	test.TestRoute(t, handler, false, "POST", "/api/composer-koji/v1/compose", `
	{
		"name":"foo",
		"version":"1",
		"release":"2",
		"distribution":"fedora-30",
		"image_requests": [
			{
				"architecture": "x86_64",
				"image_type": "qcow2",
				"repositories": [
					{
						"baseurl": "https://repo.example.com/"
					}
				]
			},
			{
				"architecture": "x86_64",
				"image_type": "qcow2",
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
	}`, http.StatusCreated, `{"koji_build_id":42}`, "id")
	wg.Wait()

	token, _, jobType, rawJob, _, err := workerServer.RequestJob(context.Background(), "x86_64", []string{"osbuild-koji"})
	require.NoError(t, err)
	require.Equal(t, "osbuild-koji", jobType)

	var osbuildJob worker.OSBuildKojiJob
	err = json.Unmarshal(rawJob, &osbuildJob)
	require.NoError(t, err)
	require.Equal(t, "koji.example.com", osbuildJob.KojiServer)
	require.Equal(t, "test.img", osbuildJob.ImageName)
	require.NotEmpty(t, osbuildJob.KojiDirectory)

	test.TestRoute(t, workerServer, false, "PATCH", fmt.Sprintf("/api/worker/v1/jobs/%v", token), `{
		"result": {
			"arch": "x86_64",
			"host_os": "fedora-30",
			"image_hash": "browns",
			"image_size": 42,
			"osbuild_output": {
				"success": true
			}
		}
	}`, http.StatusOK, `{}`)

	token, _, jobType, rawJob, _, err = workerServer.RequestJob(context.Background(), "x86_64", []string{"osbuild-koji"})
	require.NoError(t, err)
	require.Equal(t, "osbuild-koji", jobType)

	err = json.Unmarshal(rawJob, &osbuildJob)
	require.NoError(t, err)
	require.Equal(t, "koji.example.com", osbuildJob.KojiServer)
	require.Equal(t, "test.img", osbuildJob.ImageName)
	require.NotEmpty(t, osbuildJob.KojiDirectory)

	test.TestRoute(t, workerServer, false, "PATCH", fmt.Sprintf("/api/worker/v1/jobs/%v", token), `{
		"result": {
			"arch": "x86_64",
			"host_os": "fedora-30",
			"image_hash": "browns",
			"image_size": 42,
			"osbuild_output": {
				"success": true
			}
		}
	}`, http.StatusOK, `{}`)

	token, finalizeID, jobType, rawJob, _, err := workerServer.RequestJob(context.Background(), "x86_64", []string{"koji-finalize"})
	require.NoError(t, err)
	require.Equal(t, "koji-finalize", jobType)

	var kojiFinalizeJob worker.KojiFinalizeJob
	err = json.Unmarshal(rawJob, &kojiFinalizeJob)
	require.NoError(t, err)
	require.Equal(t, "koji.example.com", kojiFinalizeJob.Server)
	require.Equal(t, "1", kojiFinalizeJob.Version)
	require.Equal(t, "2", kojiFinalizeJob.Release)
	require.ElementsMatch(t, []string{"foo-1-2.x86_64.img", "foo-1-2.x86_64.img"}, kojiFinalizeJob.KojiFilenames)
	require.NotEmpty(t, kojiFinalizeJob.KojiDirectory)

	test.TestRoute(t, workerServer, false, "PATCH", fmt.Sprintf("/api/worker/v1/jobs/%v", token), `{
		"result": {
		}
	}`, http.StatusOK, `{}`)

	test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/composer-koji/v1/compose/%v", finalizeID), ``, http.StatusOK, `{
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
	}`)
}

func TestRequest(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-kojiapi-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	server, _ := newTestKojiServer(t, dir)
	handler := server.Handler("/api/composer-koji/v1")

	// Make request to an invalid route
	req := httptest.NewRequest("GET", "/invalidroute", nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	resp := rec.Result()

	var status api.Status
	err = json.NewDecoder(resp.Body).Decode(&status)
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
