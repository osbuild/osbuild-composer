package worker_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/fedoratest"
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
	test.TestRoute(t, handler, false, "GET", "/api/worker/v1/status", ``, http.StatusOK, `{"status":"OK"}`, "message")
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
		test.TestRoute(t, handler, false, c.Method, c.Path, c.Body, c.ExpectedStatus, "{}", "message")
	}
}

func TestCreate(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "worker-tests-")
	require.NoError(t, err)
	defer os.RemoveAll(tempdir)

	distroStruct := fedoratest.New()
	arch, err := distroStruct.GetArch("x86_64")
	if err != nil {
		t.Fatalf("error getting arch from distro")
	}
	imageType, err := arch.GetImageType("qcow2")
	if err != nil {
		t.Fatalf("error getting image type from arch")
	}
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, nil, 0)
	if err != nil {
		t.Fatalf("error creating osbuild manifest")
	}
	server := newTestServer(t, tempdir)
	handler := server.Handler()

	_, err = server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: manifest})
	require.NoError(t, err)

	test.TestRoute(t, handler, false, "POST", "/api/worker/v1/jobs", `{"types":["osbuild"],"arch":"x86_64"}`, http.StatusCreated,
		`{"type":"osbuild","args":{"manifest":{"pipeline":{},"sources":{}}}}`, "id", "location", "artifact_location")
}

func TestCancel(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "worker-tests-")
	require.NoError(t, err)
	defer os.RemoveAll(tempdir)

	distroStruct := fedoratest.New()
	arch, err := distroStruct.GetArch("x86_64")
	if err != nil {
		t.Fatalf("error getting arch from distro")
	}
	imageType, err := arch.GetImageType("qcow2")
	if err != nil {
		t.Fatalf("error getting image type from arch")
	}
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, nil, 0)
	if err != nil {
		t.Fatalf("error creating osbuild manifest")
	}
	server := newTestServer(t, tempdir)
	handler := server.Handler()

	jobId, err := server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: manifest})
	require.NoError(t, err)

	token, j, typ, args, dynamicArgs, err := server.RequestJob(context.Background(), arch.Name(), []string{"osbuild"})
	require.NoError(t, err)
	require.Equal(t, jobId, j)
	require.Equal(t, "osbuild", typ)
	require.NotNil(t, args)
	require.Nil(t, dynamicArgs)

	test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/worker/v1/jobs/%s", token), `{}`, http.StatusOK,
		`{"canceled":false}`)

	err = server.Cancel(jobId)
	require.NoError(t, err)

	test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/worker/v1/jobs/%s", token), `{}`, http.StatusOK,
		`{"canceled":true}`)
}

func TestUpdate(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "worker-tests-")
	require.NoError(t, err)
	defer os.RemoveAll(tempdir)

	distroStruct := fedoratest.New()
	arch, err := distroStruct.GetArch("x86_64")
	if err != nil {
		t.Fatalf("error getting arch from distro")
	}
	imageType, err := arch.GetImageType("qcow2")
	if err != nil {
		t.Fatalf("error getting image type from arch")
	}
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, nil, 0)
	if err != nil {
		t.Fatalf("error creating osbuild manifest")
	}
	server := newTestServer(t, tempdir)
	handler := server.Handler()

	jobId, err := server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: manifest})
	require.NoError(t, err)

	token, j, typ, args, dynamicArgs, err := server.RequestJob(context.Background(), arch.Name(), []string{"osbuild"})
	require.NoError(t, err)
	require.Equal(t, jobId, j)
	require.Equal(t, "osbuild", typ)
	require.NotNil(t, args)
	require.Nil(t, dynamicArgs)

	test.TestRoute(t, handler, false, "PATCH", fmt.Sprintf("/api/worker/v1/jobs/%s", token), `{}`, http.StatusOK, `{}`)
	test.TestRoute(t, handler, false, "PATCH", fmt.Sprintf("/api/worker/v1/jobs/%s", token), `{}`, http.StatusNotFound, `*`)
}

func TestArgs(t *testing.T) {
	distroStruct := fedoratest.New()
	arch, err := distroStruct.GetArch("x86_64")
	require.NoError(t, err)
	imageType, err := arch.GetImageType("qcow2")
	require.NoError(t, err)
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, nil, 0)
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

	distroStruct := fedoratest.New()
	arch, err := distroStruct.GetArch("x86_64")
	if err != nil {
		t.Fatalf("error getting arch from distro")
	}
	imageType, err := arch.GetImageType("qcow2")
	if err != nil {
		t.Fatalf("error getting image type from arch")
	}
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, nil, 0)
	if err != nil {
		t.Fatalf("error creating osbuild manifest")
	}
	server := newTestServer(t, tempdir)
	handler := server.Handler()

	jobID, err := server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: manifest})
	require.NoError(t, err)

	token, j, typ, args, dynamicArgs, err := server.RequestJob(context.Background(), arch.Name(), []string{"osbuild"})
	require.NoError(t, err)
	require.Equal(t, jobID, j)
	require.Equal(t, "osbuild", typ)
	require.NotNil(t, args)
	require.Nil(t, dynamicArgs)

	test.TestRoute(t, handler, false, "PUT", fmt.Sprintf("/api/worker/v1/jobs/%s/artifacts/foobar", token), `this is my artifact`, http.StatusOK, `?`)
}
