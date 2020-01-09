package rcm_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/google/uuid"
	test_distro "github.com/osbuild/osbuild-composer/internal/distro/test"
	"github.com/osbuild/osbuild-composer/internal/rcm"
	"github.com/osbuild/osbuild-composer/internal/store"
)

type API interface {
	ServeHTTP(writer http.ResponseWriter, request *http.Request)
}

func internalRequest(api API, method, path, body, contentType string) *http.Response {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	api.ServeHTTP(resp, req)

	return resp.Result()
}

func TestBasicRcmAPI(t *testing.T) {
	// Test the HTTP API responses
	// This test mainly focuses on HTTP status codes and JSON structures, not necessarily on their content

	var cases = []struct {
		Method            string
		Path              string
		Body              string
		ContentType       string
		ExpectedStatus    int
		ExpectedBodyRegex string
	}{
		{"GET", "/v1/compose", ``, "", http.StatusMethodNotAllowed, ``},
		{"POST", "/v1/compose", `{"status":"RUNNING"}`, "application/json", http.StatusBadRequest, ``},
		{"POST", "/v1/compose", `{"status":"RUNNING"}`, "text/plain", http.StatusBadRequest, ``},
		{"POST", "/v1/compose", `{"distribution": "fedora-30", "image_types": ["test_output"], "architectures":["test_arch"], "repositories": [{"url": "aaa", "checksum": "bbb"}]}`, "application/json", http.StatusOK, `{"compose_id":".*"}`},
		{"POST", "/v1/compose/111-222-333", `{"status":"RUNNING"}`, "application/json", http.StatusMethodNotAllowed, ``},
		{"GET", "/v1/compose/7802c476-9cd1-41b7-ba81-43c1906bce73", `{"status":"RUNNING"}`, "application/json", http.StatusBadRequest, `{"error_reason":"Compose UUID does not exist"}`},
	}

	distro := test_distro.New()
	api := rcm.New(nil, store.New(nil, distro))

	for _, c := range cases {
		resp := internalRequest(api, c.Method, c.Path, c.Body, c.ContentType)
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		response_body := buf.String()
		if resp.StatusCode != c.ExpectedStatus {
			t.Errorf("%s request to %s should return status code %d but returns %d, response: %s", c.Method, c.Path, c.ExpectedStatus, resp.StatusCode, response_body)
		}
		matched, err := regexp.Match(c.ExpectedBodyRegex, []byte(response_body))
		if err != nil {
			t.Fatalf("Failed to match regex, correct the test definition!")
		}
		if !matched {
			t.Errorf("The response to %s request to %s should match this regex %s but returns %s", c.Method, c.Path, c.ExpectedBodyRegex, response_body)
		}
	}
}

func TestSubmitCompose(t *testing.T) {
	// Test the most basic use case: Submit a new job and get its status.
	distro := test_distro.New()
	api := rcm.New(nil, store.New(nil, distro))

	var submit_reply struct {
		UUID uuid.UUID `json:"compose_id"`
	}
	var status_reply struct {
		Status      string `json:"status,omitempty"`
		ErrorReason string `json:"error_reason,omitempty"`
	}
	// Submit job
	t.Log("RCM API submit compose")
	resp := internalRequest(api, "POST", "/v1/compose", `{"distribution": "fedora-30", "image_types": ["test_output"], "architectures":["test_arch"], "repositories": [{"url": "aaa", "checksum": "bbb"}]}`, "application/json")
	if resp.StatusCode != http.StatusOK {
		t.Fatal("Failed to call /v1/compose, HTTP status code:", resp.StatusCode)
	}
	decoder := json.NewDecoder(resp.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&submit_reply)
	if err != nil {
		t.Fatal("Failed to decode response to /v1/compose:", err)
	}
	// Get the status
	t.Log("RCM API get status")
	resp = internalRequest(api, "GET", "/v1/compose/"+submit_reply.UUID.String(), "", "")
	decoder = json.NewDecoder(resp.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&status_reply)
	if err != nil {
		t.Fatal("Failed to decode response to /v1/compose/UUID:", err)
	}
	if status_reply.ErrorReason != "" {
		t.Error("Failed to get compose status, reason:", status_reply.ErrorReason)
	}
}
