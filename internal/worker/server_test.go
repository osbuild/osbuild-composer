package worker_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/fedoratest"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/testjobqueue"
	"github.com/osbuild/osbuild-composer/internal/test"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

// Ensure that the status request returns OK.
func TestStatus(t *testing.T) {
	server := worker.NewServer(nil, testjobqueue.New())
	test.TestRoute(t, server, false, "GET", "/api/worker/v1/status", ``, http.StatusOK, `{"status":"OK"}`, "message")
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

	for _, c := range cases {
		server := worker.NewServer(nil, testjobqueue.New())
		test.TestRoute(t, server, false, c.Method, c.Path, c.Body, c.ExpectedStatus, "{}", "message")
	}
}

func TestCreate(t *testing.T) {
	distroStruct := fedoratest.New()
	arch, err := distroStruct.GetArch("x86_64")
	if err != nil {
		t.Fatalf("error getting arch from distro")
	}
	imageType, err := arch.GetImageType("qcow2")
	if err != nil {
		t.Fatalf("error getting image type from arch")
	}
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, nil)
	if err != nil {
		t.Fatalf("error creating osbuild manifest")
	}
	server := worker.NewServer(nil, testjobqueue.New())

	_, err = server.Enqueue(arch.Name(), &worker.OSBuildJob{Manifest: manifest})
	require.NoError(t, err)

	test.TestRoute(t, server, false, "POST", "/api/worker/v1/jobs", `{"types":["osbuild"],"arch":"x86_64"}`, http.StatusCreated,
		`{"type":"osbuild","args":{"manifest":{"pipeline":{},"sources":{}}}}`, "id", "location", "artifact_location")
}

func TestCancel(t *testing.T) {
	distroStruct := fedoratest.New()
	arch, err := distroStruct.GetArch("x86_64")
	if err != nil {
		t.Fatalf("error getting arch from distro")
	}
	imageType, err := arch.GetImageType("qcow2")
	if err != nil {
		t.Fatalf("error getting image type from arch")
	}
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, nil)
	if err != nil {
		t.Fatalf("error creating osbuild manifest")
	}
	server := worker.NewServer(nil, testjobqueue.New())

	jobId, err := server.Enqueue(arch.Name(), &worker.OSBuildJob{Manifest: manifest})
	require.NoError(t, err)

	token, j, _, err := server.RequestOSBuildJob(context.Background(), arch.Name())
	require.NoError(t, err)
	require.Equal(t, jobId, j)

	err = server.Cancel(jobId)
	require.NoError(t, err)

	test.TestRoute(t, server, false, "GET", fmt.Sprintf("/api/worker/v1/jobs/%s", token), `{}`, http.StatusOK,
		`{"canceled":true}`)
}
