package jobqueue_test

import (
	"net/http"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	test_distro "github.com/osbuild/osbuild-composer/internal/distro/test"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/test"

	"github.com/google/uuid"
)

func TestBasic(t *testing.T) {
	var cases = []struct {
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		// Create job with invalid body
		{"POST", "/job-queue/v1/jobs", ``, http.StatusBadRequest, ``},
		// Update job with invalid ID
		{"PATCH", "/job-queue/v1/jobs/foo", `{"status":"RUNNING"}`, http.StatusBadRequest, ``},
		// Update job that does not exist, with invalid body
		{"PATCH", "/job-queue/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ``, http.StatusBadRequest, ``},
		// Update job that does not exist
		{"PATCH", "/job-queue/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", `{"status":"RUNNING"}`, http.StatusNotFound, ``},
	}

	for _, c := range cases {
		distro := test_distro.New()
		api := jobqueue.New(nil, store.New(nil, distro))

		test.TestRoute(t, api, false, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
	}
}

func TestCreate(t *testing.T) {
	id, _ := uuid.Parse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	distro := test_distro.New()
	store := store.New(nil, distro)
	api := jobqueue.New(nil, store)

	err := store.PushCompose(id, &blueprint.Blueprint{}, map[string]string{"test-repo": "test:foo"}, "test_arch", "test_output", nil)
	if err != nil {
		t.Fatalf("error pushing compose: %v", err)
	}

	test.TestRoute(t, api, false, "POST", "/job-queue/v1/jobs", `{}`, http.StatusCreated,
		`{"distro":"test-distro","id":"ffffffff-ffff-ffff-ffff-ffffffffffff","output_type":"test_output","pipeline":{},"targets":[]}`, "created", "uuid")
}

func testUpdateTransition(t *testing.T, from, to string, expectedStatus int) {
	id, _ := uuid.Parse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	distro := test_distro.New()
	store := store.New(nil, distro)
	api := jobqueue.New(nil, store)

	if from != "VOID" {
		err := store.PushCompose(id, &blueprint.Blueprint{}, map[string]string{"test": "test:foo"}, "test_arch", "test_output", nil)
		if err != nil {
			t.Fatalf("error pushing compose: %v", err)
		}
		if from != "WAITING" {
			test.SendHTTP(api, false, "POST", "/job-queue/v1/jobs", `{}`)
			if from != "RUNNING" {
				test.SendHTTP(api, false, "PATCH", "/job-queue/v1/jobs/ffffffff-ffff-ffff-ffff-ffffffffffff", `{"status":"`+from+`"}`)
			}
		}
	}

	test.TestRoute(t, api, false, "PATCH", "/job-queue/v1/jobs/ffffffff-ffff-ffff-ffff-ffffffffffff", `{"status":"`+to+`"}`, expectedStatus, ``)
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
