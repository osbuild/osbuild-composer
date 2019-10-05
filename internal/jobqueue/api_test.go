package jobqueue_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"osbuild-composer/internal/blueprint"
	"osbuild-composer/internal/jobqueue"
	"osbuild-composer/internal/store"

	"github.com/google/uuid"
)

func testRoute(t *testing.T, api *jobqueue.API, method, path, body string, expectedStatus int, expectedJSON string) {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	api.ServeHTTP(resp, req)

	if resp.Code != expectedStatus {
		t.Errorf("%s: expected status %v, but got %v", path, expectedStatus, resp.Code)
		return
	}

	replyJSON, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("%s: could not read reponse body: %v", path, err)
		return
	}

	if expectedJSON == "" {
		if len(replyJSON) != 0 {
			t.Errorf("%s: expected no response body, but got:\n%s", path, replyJSON)
		}
		return
	}

	var reply, expected interface{}
	err = json.Unmarshal(replyJSON, &reply)
	if err != nil {
		t.Errorf("%s: %v\n%s", path, err, string(replyJSON))
		return
	}

	if expectedJSON == "*" {
		return
	}

	err = json.Unmarshal([]byte(expectedJSON), &expected)
	if err != nil {
		t.Errorf("%s: expected JSON is invalid: %v", path, err)
		return
	}

	if !reflect.DeepEqual(reply, expected) {
		t.Errorf("%s: reply != expected:\n   reply: %s\nexpected: %s", path, strings.TrimSpace(string(replyJSON)), expectedJSON)
		return
	}
}

func TestBasic(t *testing.T) {
	expected_job := `{"id":"ffffffff-ffff-ffff-ffff-ffffffffffff","pipeline":{"assembler":{"name":"org.osbuild.tar","options":{"filename":"image.tar"}}},"targets":[{"name":"org.osbuild.local","options":{"location":"/var/lib/osbuild-composer/outputs/ffffffff-ffff-ffff-ffff-ffffffffffff"}}]}`
	var cases = []struct {
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{"POST", "/job-queue/v1/foo", ``, http.StatusNotFound, ``},
		{"GET", "/job-queue/v1/foo", ``, http.StatusNotFound, ``},
		{"PATH", "/job-queue/v1/foo", ``, http.StatusNotFound, ``},
		{"DELETE", "/job-queue/v1/foo", ``, http.StatusNotFound, ``},

		{"POST", "/job-queue/v1/jobs", `{"id":"ffffffff-ffff-ffff-ffff-ffffffffffff"}`, http.StatusCreated, expected_job},
		//{"POST", "/job-queue/v1/jobs", `{"id":"ffffffff-ffff-ffff-ffff-ffffffffffff"}`, http.StatusBadRequest, ``},
		//{"PATCH", "/job-queue/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", `{"status":"FINISHED"}`, http.StatusBadRequest, ``},
		{"PATCH", "/job-queue/v1/jobs/ffffffff-ffff-ffff-ffff-ffffffffffff", `{"status":"RUNNING"}`, http.StatusOK, ``},
		{"PATCH", "/job-queue/v1/jobs/ffffffff-ffff-ffff-ffff-ffffffffffff", `{"status":"RUNNING"}`, http.StatusOK, ``},
		{"PATCH", "/job-queue/v1/jobs/ffffffff-ffff-ffff-ffff-ffffffffffff", `{"status":"FINISHED"}`, http.StatusOK, ``},
		//{"PATCH", "/job-queue/v1/jobs/ffffffff-ffff-ffff-ffff-ffffffffffff", `{"status":"RUNNING"}`, http.StatusNotAllowed, ``},
		//{"PATCH", "/job-queue/v1/jobs/ffffffff-ffff-ffff-ffff-ffffffffffff", `{"status":"FINISHED"}`, http.StatusNotAllowed, ``},
	}

	store := store.New(nil, nil)
	api := jobqueue.New(nil, store)
	for _, c := range cases {
		id, _ := uuid.Parse("ffffffff-ffff-ffff-ffff-ffffffffffff")
		store.PushCompose(id, &blueprint.Blueprint{}, "tar")

		testRoute(t, api, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
	}
}
