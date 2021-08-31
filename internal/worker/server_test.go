package worker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/test_distro"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	"github.com/osbuild/osbuild-composer/internal/test"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

func newTestServer(t *testing.T, tempdir string) *worker.Server {
	q, err := fsjobqueue.New(tempdir)
	if err != nil {
		t.Fatalf("error creating fsjobqueue: %v", err)
	}
	return worker.NewServer(nil, q, "")
}

// Ensure that the status request returns OK.
func TestStatus(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "worker-tests-")
	require.NoError(t, err)
	defer os.RemoveAll(tempdir)

	server := newTestServer(t, tempdir)
	handler := server.Handler()
	test.TestRoute(t, handler, false, "GET", "/api/worker/v1/status", ``, http.StatusOK, `{"status":"OK", "href": "/api/worker/v1/status", "kind":"Status"}`, "message", "id")
}

func TestErrors(t *testing.T) {
	var cases = []struct {
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
	}{
		// Bogus path
		{"GET", "/api/worker/v1/foo", ``, http.StatusNotFound},
		// Create job with invalid body
		{"POST", "/api/worker/v1/jobs", ``, http.StatusBadRequest},
		// Wrong method
		{"GET", "/api/worker/v1/jobs", ``, http.StatusMethodNotAllowed},
		// Update job with invalid ID
		{"PATCH", "/api/worker/v1/jobs/foo", `{"status":"FINISHED"}`, http.StatusBadRequest},
		// Update job that does not exist, with invalid body
		{"PATCH", "/api/worker/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ``, http.StatusBadRequest},
		// Update job that does not exist
		{"PATCH", "/api/worker/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", `{"status":"FINISHED"}`, http.StatusNotFound},
	}

	tempdir, err := ioutil.TempDir("", "worker-tests-")
	require.NoError(t, err)
	defer os.RemoveAll(tempdir)

	for _, c := range cases {
		server := newTestServer(t, tempdir)
		handler := server.Handler()
		test.TestRoute(t, handler, false, c.Method, c.Path, c.Body, c.ExpectedStatus, `{"kind":"Error"}`, "message", "href", "operation_id", "reason", "id", "code")
	}
}

func TestCreate(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "worker-tests-")
	require.NoError(t, err)
	defer os.RemoveAll(tempdir)

	distroStruct := test_distro.New()
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	imageType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		t.Fatalf("error getting image type from arch: %v", err)
	}
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, 0)
	if err != nil {
		t.Fatalf("error creating osbuild manifest: %v", err)
	}
	server := newTestServer(t, tempdir)
	handler := server.Handler()

	_, err = server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: manifest})
	require.NoError(t, err)

	test.TestRoute(t, handler, false, "POST", "/api/worker/v1/jobs",
		fmt.Sprintf(`{"types":["osbuild"],"arch":"%s"}`, test_distro.TestArchName), http.StatusCreated,
		`{"kind":"RequestJob","href":"/api/worker/v1/jobs","type":"osbuild","args":{"manifest":{"pipeline":{},"sources":{}}}}`, "id", "location", "artifact_location")
}

func TestCancel(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "worker-tests-")
	require.NoError(t, err)
	defer os.RemoveAll(tempdir)

	distroStruct := test_distro.New()
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	imageType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		t.Fatalf("error getting image type from arch: %v", err)
	}
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, 0)
	if err != nil {
		t.Fatalf("error creating osbuild manifest: %v", err)
	}
	server := newTestServer(t, tempdir)
	handler := server.Handler()

	jobId, err := server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: manifest})
	require.NoError(t, err)

	j, token, typ, args, dynamicArgs, err := server.RequestJob(context.Background(), arch.Name(), []string{"osbuild"})
	require.NoError(t, err)
	require.Equal(t, jobId, j)
	require.Equal(t, "osbuild", typ)
	require.NotNil(t, args)
	require.Nil(t, dynamicArgs)

	test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/worker/v1/jobs/%s", token), `{}`, http.StatusOK,
		fmt.Sprintf(`{"canceled":false,"href":"/api/worker/v1/jobs/%s","id":"%s","kind":"JobStatus"}`, token, token))

	err = server.Cancel(jobId)
	require.NoError(t, err)

	test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/worker/v1/jobs/%s", token), `{}`, http.StatusOK,
		fmt.Sprintf(`{"canceled":true,"href":"/api/worker/v1/jobs/%s","id":"%s","kind":"JobStatus"}`, token, token))
}

func TestUpdate(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "worker-tests-")
	require.NoError(t, err)
	defer os.RemoveAll(tempdir)

	distroStruct := test_distro.New()
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	imageType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		t.Fatalf("error getting image type from arch: %v", err)
	}
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, 0)
	if err != nil {
		t.Fatalf("error creating osbuild manifest: %v", err)
	}
	server := newTestServer(t, tempdir)
	handler := server.Handler()

	jobId, err := server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: manifest})
	require.NoError(t, err)

	j, token, typ, args, dynamicArgs, err := server.RequestJob(context.Background(), arch.Name(), []string{"osbuild"})
	require.NoError(t, err)
	require.Equal(t, jobId, j)
	require.Equal(t, "osbuild", typ)
	require.NotNil(t, args)
	require.Nil(t, dynamicArgs)

	test.TestRoute(t, handler, false, "PATCH", fmt.Sprintf("/api/worker/v1/jobs/%s", token), `{}`, http.StatusOK,
		fmt.Sprintf(`{"href":"/api/worker/v1/jobs/%s","id":"%s","kind":"UpdateJobResponse"}`, token, token))
	test.TestRoute(t, handler, false, "PATCH", fmt.Sprintf("/api/worker/v1/jobs/%s", token), `{}`, http.StatusNotFound,
		`{"href":"/api/composer-worker/v1/errors/5","code":"COMPOSER-WORKER-5","id":"5","kind":"Error","message":"Token not found","reason":"Token not found"}`,
		"operation_id")
}

func TestArgs(t *testing.T) {
	distroStruct := test_distro.New()
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	require.NoError(t, err)
	imageType, err := arch.GetImageType(test_distro.TestImageTypeName)
	require.NoError(t, err)
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, 0)
	require.NoError(t, err)

	tempdir, err := ioutil.TempDir("", "worker-tests-")
	require.NoError(t, err)
	defer os.RemoveAll(tempdir)
	server := newTestServer(t, tempdir)

	job := worker.OSBuildJob{
		Manifest:  manifest,
		ImageName: "test-image",
	}
	jobId, err := server.EnqueueOSBuild(arch.Name(), &job)
	require.NoError(t, err)

	_, _, _, args, _, err := server.RequestJob(context.Background(), arch.Name(), []string{"osbuild"})
	require.NoError(t, err)
	require.NotNil(t, args)

	var jobArgs worker.OSBuildJob
	jobType, rawArgs, deps, err := server.Job(jobId, &jobArgs)
	require.NoError(t, err)
	require.Equal(t, args, rawArgs)
	require.Equal(t, job, jobArgs)
	require.Equal(t, jobType, "osbuild:"+arch.Name())
	require.Equal(t, []uuid.UUID(nil), deps)
}

func TestUpload(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "worker-tests-")
	require.NoError(t, err)
	defer os.RemoveAll(tempdir)

	distroStruct := test_distro.New()
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	imageType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		t.Fatalf("error getting image type from arch: %v", err)
	}
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, 0)
	if err != nil {
		t.Fatalf("error creating osbuild manifest: %v", err)
	}
	server := newTestServer(t, tempdir)
	handler := server.Handler()

	jobID, err := server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: manifest})
	require.NoError(t, err)

	j, token, typ, args, dynamicArgs, err := server.RequestJob(context.Background(), arch.Name(), []string{"osbuild"})
	require.NoError(t, err)
	require.Equal(t, jobID, j)
	require.Equal(t, "osbuild", typ)
	require.NotNil(t, args)
	require.Nil(t, dynamicArgs)

	test.TestRoute(t, handler, false, "PUT", fmt.Sprintf("/api/worker/v1/jobs/%s/artifacts/foobar", token), `this is my artifact`, http.StatusOK, `?`)
}

func TestOAuth(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "worker-tests-")
	require.NoError(t, err)
	defer os.RemoveAll(tempdir)

	q, err := fsjobqueue.New(tempdir)
	require.NoError(t, err)
	workerServer := worker.NewServer(nil, q, tempdir)
	handler := workerServer.Handler()

	workSrv := httptest.NewServer(handler)
	defer workSrv.Close()

	/* Check that the worker supplies the access token  */
	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer accessToken!", r.Header.Get("Authorization"))
		handler.ServeHTTP(w, r)
	}))
	defer proxySrv.Close()

	offlineToken := "someOfflineToken"
	/* Start oauth srv supplying the bearer token */
	oauthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		err = r.ParseForm()
		require.NoError(t, err)

		require.Equal(t, "refresh_token", r.FormValue("grant_type"))
		require.Equal(t, "rhsm-api", r.FormValue("client_id"))
		require.Equal(t, offlineToken, r.FormValue("refresh_token"))

		bt := struct {
			AccessToken     string `json:"access_token"`
			ValidForSeconds int    `json:"expires_in"`
		}{
			"accessToken!",
			900,
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(bt)
		require.NoError(t, err)
	}))
	defer oauthSrv.Close()

	distroStruct := test_distro.New()
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	imageType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		t.Fatalf("error getting image type from arch: %v", err)
	}
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, 0)
	if err != nil {
		t.Fatalf("error creating osbuild manifest: %v", err)
	}

	_, err = workerServer.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: manifest})
	require.NoError(t, err)

	client, err := worker.NewClient(proxySrv.URL, nil, &offlineToken, &oauthSrv.URL)
	require.NoError(t, err)
	job, err := client.RequestJob([]string{"osbuild"}, arch.Name())
	require.NoError(t, err)
	r := strings.NewReader("artifact contents")
	require.NoError(t, job.UploadArtifact("some-artifact", r))
	c, err := job.Canceled()
	require.False(t, c)
}
