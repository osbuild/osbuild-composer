package jobqueue_test

import (
	"net/http"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	test_distro "github.com/osbuild/osbuild-composer/internal/distro/fedoratest"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/test"

	"github.com/google/uuid"
)

func TestBasic(t *testing.T) {
	var cases = []struct {
		Method           string
		Path             string
		Body             string
		ExpectedStatus   int
		ExpectedResponse string
	}{
		// Create job with invalid body
		{"POST", "/job-queue/v1/jobs", ``, http.StatusBadRequest, `invalid request: EOF`},
		// Update job with invalid ID
		{"PATCH", "/job-queue/v1/jobs/foo/builds/0", `{"status":"RUNNING"}`, http.StatusBadRequest, `invalid compose id: invalid UUID length: 3`},
		// Update job that does not exist, with invalid body
		{"PATCH", "/job-queue/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/builds/0", ``, http.StatusBadRequest, `invalid status: EOF`},
		// Update job that does not exist
		{"PATCH", "/job-queue/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/builds/0", `{"status":"RUNNING"}`, http.StatusNotFound, `compose does not exist`},
	}

	for _, c := range cases {
		api := jobqueue.New(nil, store.New(nil))

		test.TestNonJsonRoute(t, api, false, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedResponse)
	}
}

func TestCreate(t *testing.T) {
	id, _ := uuid.Parse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	distroStruct := test_distro.New()
	store := store.New(nil)
	api := jobqueue.New(nil, store)

	err := store.PushCompose(distroStruct, id, &blueprint.Blueprint{}, nil, nil, nil, "x86_64", "qcow2", 0, nil)
	if err != nil {
		t.Fatalf("error pushing compose: %v", err)
	}

	test.TestRoute(t, api, false, "POST", "/job-queue/v1/jobs", `{}`, http.StatusCreated,
		`{"id":"ffffffff-ffff-ffff-ffff-ffffffffffff","image_build_id":0,"manifest":{"sources":{},"pipeline":{}},"targets":[],"output_type":"qcow2"}`, "created", "uuid")
}

func testUpdateTransition(t *testing.T, from, to string, expectedStatus int, expectedResponse string) {
	id, _ := uuid.Parse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	distroStruct := test_distro.New()
	store := store.New(nil)
	api := jobqueue.New(nil, store)

	if from != "VOID" {
		err := store.PushCompose(distroStruct, id, &blueprint.Blueprint{}, nil, nil, nil, "x86_64", "qcow2", 0, nil)
		if err != nil {
			t.Fatalf("error pushing compose: %v", err)
		}
		if from != "WAITING" {
			test.SendHTTP(api, false, "POST", "/job-queue/v1/jobs", `{}`)
			if from != "RUNNING" {
				test.SendHTTP(api, false, "PATCH", "/job-queue/v1/jobs/ffffffff-ffff-ffff-ffff-ffffffffffff/builds/0", `{"status":"`+from+`"}`)
			}
		}
	}

	test.TestNonJsonRoute(t, api, false, "PATCH", "/job-queue/v1/jobs/ffffffff-ffff-ffff-ffff-ffffffffffff/builds/0", `{"status":"`+to+`"}`, expectedStatus, expectedResponse)
}

func TestUpdate(t *testing.T) {
	var cases = []struct {
		From             string
		To               string
		ExpectedStatus   int
		ExpectedResponse string
	}{
		{"VOID", "WAITING", http.StatusNotFound, "compose does not exist"},
		{"VOID", "RUNNING", http.StatusNotFound, "compose does not exist"},
		{"VOID", "FINISHED", http.StatusNotFound, "compose does not exist"},
		{"VOID", "FAILED", http.StatusNotFound, "compose does not exist"},
		{"WAITING", "WAITING", http.StatusNotFound, "compose has not been popped"},
		{"WAITING", "RUNNING", http.StatusNotFound, "compose has not been popped"},
		{"WAITING", "FINISHED", http.StatusNotFound, "compose has not been popped"},
		{"WAITING", "FAILED", http.StatusNotFound, "compose has not been popped"},
		{"RUNNING", "WAITING", http.StatusBadRequest, "invalid state transition: image build cannot be moved into waiting state"},
		{"RUNNING", "RUNNING", http.StatusOK, ""},
		{"RUNNING", "FINISHED", http.StatusOK, ""},
		{"RUNNING", "FAILED", http.StatusOK, ""},
		{"FINISHED", "WAITING", http.StatusBadRequest, "invalid state transition: image build cannot be moved into waiting state"},
		{"FINISHED", "RUNNING", http.StatusBadRequest, "invalid state transition: only waiting image build can be transitioned into running state"},
		{"FINISHED", "FINISHED", http.StatusBadRequest, "invalid state transition: only running image build can be transitioned into finished or failed state"},
		{"FINISHED", "FAILED", http.StatusBadRequest, "invalid state transition: only running image build can be transitioned into finished or failed state"},
		{"FAILED", "WAITING", http.StatusBadRequest, "invalid state transition: image build cannot be moved into waiting state"},
		{"FAILED", "RUNNING", http.StatusBadRequest, "invalid state transition: only waiting image build can be transitioned into running state"},
		{"FAILED", "FINISHED", http.StatusBadRequest, "invalid state transition: only running image build can be transitioned into finished or failed state"},
		{"FAILED", "FAILED", http.StatusBadRequest, "invalid state transition: only running image build can be transitioned into finished or failed state"},
	}

	for _, c := range cases {
		testUpdateTransition(t, c.From, c.To, c.ExpectedStatus, c.ExpectedResponse)
	}
}
