package rcm_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/google/uuid"

	distro_mock "github.com/osbuild/osbuild-composer/internal/mocks/distro"
	rpmmd_mock "github.com/osbuild/osbuild-composer/internal/mocks/rpmmd"
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
		{"POST", "/v1/compose", `{"distribution": "fedora-30", "image_types": ["qcow2"], "architectures":["x86_64"], "repositories": []}`, "application/json", http.StatusBadRequest, ""},
		{"POST", "/v1/compose/111-222-333", `{"status":"RUNNING"}`, "application/json", http.StatusMethodNotAllowed, ``},
		{"GET", "/v1/compose/7802c476-9cd1-41b7-ba81-43c1906bce73", `{"status":"RUNNING"}`, "application/json", http.StatusBadRequest, `{"error_reason":"Compose UUID does not exist"}`},
	}

	registry, err := distro_mock.NewDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	api := rcm.New(nil, store.New(nil), rpmmd_mock.NewRPMMDMock(rpmmd_mock.BaseFixture()), registry)

	for _, c := range cases {
		resp := internalRequest(api, c.Method, c.Path, c.Body, c.ContentType)
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
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
	registry, err := distro_mock.NewDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	api := rcm.New(nil, store.New(nil), rpmmd_mock.NewRPMMDMock(rpmmd_mock.BaseFixture()), registry)

	var submit_reply struct {
		UUID uuid.UUID `json:"compose_id"`
	}
	var status_reply struct {
		Status      string `json:"status,omitempty"`
		ErrorReason string `json:"error_reason,omitempty"`
	}

	var cases = []struct {
		Method      string
		Path        string
		Body        string
		ContentType string
	}{
		{
			"POST",
			"/v1/compose",
			`{"distribution": "fedora-30", 
					"image_types": ["qcow2"], 
					"architectures":["x86_64"], 
					"repositories": [{
						"url": "http://download.fedoraproject.org/pub/fedora/linux/releases/30/Everything/x86_64/os/"
					}]}`,
			"application/json",
		},
	}

	for n, c := range cases {
		// Submit job
		t.Logf("RCM API submit compose, case %d\n", n)
		resp := internalRequest(api, c.Method, c.Path, c.Body, c.ContentType)
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
}
