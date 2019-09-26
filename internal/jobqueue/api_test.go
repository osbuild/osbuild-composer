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

	"osbuild-composer/internal/job"
	"osbuild-composer/internal/jobqueue"
	"osbuild-composer/internal/pipeline"
	"osbuild-composer/internal/target"
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
	expected_job := `{"pipeline":{"assembler":{"name":"org.osbuild.tar","options":{"filename":"image.tar"}}},"targets":[{"name":"org.osbuild.local","options":{"location":"/var/lib/osbuild-composer/ffffffff-ffff–ffff-ffff-ffffffffffff"}}]}`
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

		{"POST", "/job-queue/v1/jobs", `{"id":"ffffffff-ffff–ffff-ffff-ffffffffffff"}`, http.StatusOK, expected_job},
		{"POST", "/job-queue/v1/jobs", `{"id":"ffffffff-ffff–ffff-ffff-ffffffffffff"}`, http.StatusBadRequest, ``},
		//{"PATCH", "/job-queue/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", `{"status":"finished"}`, http.StatusBadRequest, ``},
		{"PATCH", "/job-queue/v1/jobs/ffffffff-ffff-ffff-ffff-ffffffffffff", `{"status":"running"}`, http.StatusOK, ``},
		{"PATCH", "/job-queue/v1/jobs/ffffffff-ffff-ffff-ffff-ffffffffffff", `{"status":"running"}`, http.StatusOK, ``},
		{"PATCH", "/job-queue/v1/jobs/ffffffff-ffff-ffff-ffff-ffffffffffff", `{"status":"finished"}`, http.StatusOK, ``},
		//{"PATCH", "/job-queue/v1/jobs/ffffffff-ffff-ffff-ffff-ffffffffffff", `{"status":"running"}`, http.StatusNotAllowed, ``},
		//{"PATCH", "/job-queue/v1/jobs/ffffffff-ffff-ffff-ffff-ffffffffffff", `{"status":"finished"}`, http.StatusNotAllowed, ``},
	}

	jobChannel := make(chan job.Job, 100)
	api := jobqueue.New(nil, jobChannel)
	for _, c := range cases {
		jobChannel <- job.Job{
			ComposeID: "ffffffff-ffff–ffff-ffff-ffffffffffff",
			Pipeline: pipeline.Pipeline{
				Assembler: pipeline.Assembler{
					Name: "org.osbuild.tar",
					Options: pipeline.AssemblerTarOptions{
						Filename: "image.tar",
					},
				},
			},
			Targets: []target.Target{{
				Name: "org.osbuild.local",
				Options: target.LocalOptions{
					Location: "/var/lib/osbuild-composer/ffffffff-ffff–ffff-ffff-ffffffffffff",
				}},
			},
		}

		testRoute(t, api, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
	}
}
