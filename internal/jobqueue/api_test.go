package jobqueue_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro/fedoratest"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/test"
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
		{"PATCH", "/job-queue/v1/jobs/foo/builds/0", `{"status":"RUNNING"}`, http.StatusBadRequest},
		// Update job that does not exist, with invalid body
		{"PATCH", "/job-queue/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/builds/0", ``, http.StatusBadRequest},
		// Update job that does not exist
		{"PATCH", "/job-queue/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/builds/0", `{"status":"RUNNING"}`, http.StatusNotFound},
	}

	for _, c := range cases {
		api := jobqueue.New(nil, store.New(nil))
		test.TestRoute(t, api, false, c.Method, c.Path, c.Body, c.ExpectedStatus, "{}", "message")
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
	store := store.New(nil)
	api := jobqueue.New(nil, store)

	id, err := store.PushCompose(imageType, &blueprint.Blueprint{}, nil, nil, nil, 0, nil)
	if err != nil {
		t.Fatalf("error pushing compose: %v", err)
	}

	test.TestRoute(t, api, false, "POST", "/job-queue/v1/jobs", `{}`, http.StatusCreated,
		`{"compose_id":"`+id.String()+`","image_build_id":0,"manifest":{"sources":{},"pipeline":{}},"targets":[]}`, "created")
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
	store := store.New(nil)
	api := jobqueue.New(nil, store)

	id := uuid.Nil
	if from != "VOID" {
		id, err = store.PushCompose(imageType, &blueprint.Blueprint{}, nil, nil, nil, 0, nil)
		if err != nil {
			t.Fatalf("error pushing compose: %v", err)
		}

		if from != "WAITING" {
			test.SendHTTP(api, false, "POST", "/job-queue/v1/jobs", `{}`)
			if from != "RUNNING" {
				test.SendHTTP(api, false, "PATCH", "/job-queue/v1/jobs/"+id.String()+"/builds/0", `{"status":"`+from+`"}`)
			}
		}
	}

	test.TestRoute(t, api, false, "PATCH", "/job-queue/v1/jobs/"+id.String()+"/builds/0", `{"status":"`+to+`"}`, expectedStatus, "{}", "message")
}

func TestUpdate(t *testing.T) {
	var cases = []struct {
		From           string
		To             string
		ExpectedStatus int
	}{
		{"VOID", "WAITING", http.StatusNotFound},
		{"VOID", "RUNNING", http.StatusNotFound},
		{"VOID", "FINISHED", http.StatusNotFound},
		{"VOID", "FAILED", http.StatusNotFound},
		{"WAITING", "WAITING", http.StatusNotFound},
		{"WAITING", "RUNNING", http.StatusNotFound},
		{"WAITING", "FINISHED", http.StatusNotFound},
		{"WAITING", "FAILED", http.StatusNotFound},
		{"RUNNING", "WAITING", http.StatusBadRequest},
		{"RUNNING", "RUNNING", http.StatusOK},
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
		testUpdateTransition(t, c.From, c.To, c.ExpectedStatus)
	}
}
