package worker_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/distro/fedoratest"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/testjobqueue"
	"github.com/osbuild/osbuild-composer/internal/test"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

func TestErrors(t *testing.T) {
	var cases = []struct {
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
	}{
		// Bogus path
		{"GET", "/foo", ``, http.StatusNotFound},
		// Create job with invalid body
		{"POST", "/job-queue/v1/jobs", ``, http.StatusBadRequest},
		// Wrong method
		{"GET", "/job-queue/v1/jobs", ``, http.StatusMethodNotAllowed},
		// Update job with invalid ID
		{"PATCH", "/job-queue/v1/jobs/foo", `{"status":"FINISHED"}`, http.StatusBadRequest},
		// Update job that does not exist, with invalid body
		{"PATCH", "/job-queue/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ``, http.StatusBadRequest},
		// Update job that does not exist
		{"PATCH", "/job-queue/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", `{"status":"FINISHED"}`, http.StatusNotFound},
	}

	for _, c := range cases {
		server := worker.NewServer(nil, testjobqueue.New(), nil)
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
	server := worker.NewServer(nil, testjobqueue.New(), nil)

	manifest, err := imageType.Manifest(nil, nil, nil, nil, imageType.Size(0))
	if err != nil {
		t.Fatalf("error creating osbuild manifest")
	}

	id, err := server.Enqueue(manifest, nil)
	require.NoError(t, err)

	test.TestRoute(t, server, false, "POST", "/job-queue/v1/jobs", `{}`, http.StatusCreated,
		`{"id":"`+id.String()+`","manifest":{"sources":{},"pipeline":{}}}`, "created")
}

func testUpdateTransition(t *testing.T, from, to string, expectedStatus int) {
	distroStruct := fedoratest.New()
	arch, err := distroStruct.GetArch("x86_64")
	if err != nil {
		t.Fatalf("error getting arch from distro")
	}
	imageType, err := arch.GetImageType("qcow2")
	if err != nil {
		t.Fatalf("error getting image type from arch")
	}
	server := worker.NewServer(nil, testjobqueue.New(), nil)

	id := uuid.Nil
	if from != "VOID" {
		manifest, err := imageType.Manifest(nil, nil, nil, nil, imageType.Size(0))
		if err != nil {
			t.Fatalf("error creating osbuild manifest")
		}

		id, err = server.Enqueue(manifest, nil)
		require.NoError(t, err)

		if from != "WAITING" {
			test.SendHTTP(server, false, "POST", "/job-queue/v1/jobs", `{}`)
			if from != "RUNNING" {
				test.SendHTTP(server, false, "PATCH", "/job-queue/v1/jobs/"+id.String(), `{"status":"`+from+`"}`)
			}
		}
	}

	test.TestRoute(t, server, false, "PATCH", "/job-queue/v1/jobs/"+id.String(), `{"status":"`+to+`"}`, expectedStatus, "{}", "message")
}

func TestUpdate(t *testing.T) {
	var cases = []struct {
		From           string
		To             string
		ExpectedStatus int
	}{
		{"VOID", "WAITING", http.StatusBadRequest},
		{"VOID", "RUNNING", http.StatusBadRequest},
		{"VOID", "FINISHED", http.StatusNotFound},
		{"VOID", "FAILED", http.StatusNotFound},
		{"WAITING", "WAITING", http.StatusBadRequest},
		{"WAITING", "RUNNING", http.StatusBadRequest},
		{"WAITING", "FINISHED", http.StatusBadRequest},
		{"WAITING", "FAILED", http.StatusBadRequest},
		{"RUNNING", "WAITING", http.StatusBadRequest},
		{"RUNNING", "RUNNING", http.StatusBadRequest},
		{"RUNNING", "FINISHED", http.StatusOK},
		{"RUNNING", "FAILED", http.StatusOK},
		{"FINISHED", "WAITING", http.StatusBadRequest},
		{"FINISHED", "RUNNING", http.StatusBadRequest},
		{"FINISHED", "FINISHED", http.StatusBadRequest},
		{"FINISHED", "FAILED", http.StatusBadRequest},
		{"FAILED", "WAITING", http.StatusBadRequest},
		{"FAILED", "RUNNING", http.StatusBadRequest},
		{"FAILED", "FINISHED", http.StatusBadRequest},
		{"FAILED", "FAILED", http.StatusBadRequest},
	}

	for _, c := range cases {
		t.Log(c)
		testUpdateTransition(t, c.From, c.To, c.ExpectedStatus)
	}
}
