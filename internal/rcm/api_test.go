package rcm_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/jobqueue/testjobqueue"
	distro_mock "github.com/osbuild/osbuild-composer/internal/mocks/distro"
	rpmmd_mock "github.com/osbuild/osbuild-composer/internal/mocks/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/rcm"
	"github.com/osbuild/osbuild-composer/internal/worker"
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

func newTestWorkerServer(t *testing.T) (*worker.Server, string) {
	dir, err := ioutil.TempDir("", "rcm-test-")
	require.NoError(t, err)

	w := worker.NewServer(nil, testjobqueue.New(), "")
	require.NotNil(t, w)

	return w, dir
}

func cleanupTempDir(t *testing.T, dir string) {
	err := os.RemoveAll(dir)
	require.NoError(t, err)
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
		{"POST", "/v1/compose", `{"image_builds":[]}`, "application/json", http.StatusBadRequest, ""},
		{"POST", "/v1/compose/111-222-333", `{"status":"RUNNING"}`, "application/json", http.StatusMethodNotAllowed, ``},
		{"GET", "/v1/compose/7802c476-9cd1-41b7-ba81-43c1906bce73", `{"status":"RUNNING"}`, "application/json", http.StatusBadRequest, ``},
	}

	registry, err := distro_mock.NewDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}

	workers, dir := newTestWorkerServer(t)
	defer cleanupTempDir(t, dir)

	api := rcm.New(nil, workers, rpmmd_mock.NewRPMMDMock(rpmmd_mock.BaseFixture()), registry)

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

func TestSubmit(t *testing.T) {
	registry, err := distro_mock.NewDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}

	workers, dir := newTestWorkerServer(t)
	defer cleanupTempDir(t, dir)

	api := rcm.New(nil, workers, rpmmd_mock.NewRPMMDMock(rpmmd_mock.BaseFixture()), registry)

	var submit_reply struct {
		UUID uuid.UUID `json:"compose_id"`
	}

	var cases = []struct {
		Method         string
		Path           string
		Body           string
		ContentType    string
		ExpectedStatus int
	}{
		{
			"POST",
			"/v1/compose",
			`{
				"image_builds":
				[
					{
						"distribution": "fedora-30",
						"architecture": "x86_64",
						"image_type": "qcow2",
						"repositories":
						[
							{
								"baseurl": "http://mirrors.kernel.org/fedora/releases/30/Everything/x86_64/os/"
							}
						]
					}
				]
			}`,
			"application/json",
			http.StatusOK,
		},
		{
			"POST",
			"/v1/compose",
			`{
				"image_builds":
				[
					{
						"distribution": "invalid",
						"architecture": "x86_64",
						"image_type": "qcow2",
						"repositories":
						[
							{
								"baseurl": "http://mirrors.kernel.org/fedora/releases/30/Everything/x86_64/os/"
							}
						]
					}
				]
			}`,
			"application/json",
			http.StatusBadRequest,
		},
		{
			"POST",
			"/v1/compose",
			`{
				"image_builds":
				[
					{
						"distribution": "fedora-30",
						"architecture": "invalid",
						"image_type": "qcow2",
						"repositories":
						[
							{
								"baseurl": "http://mirrors.kernel.org/fedora/releases/30/Everything/x86_64/os/"
							}
						]
					}
				]
			}`,
			"application/json",
			http.StatusBadRequest,
		},
		{
			"POST",
			"/v1/compose",
			`{
				"image_builds":
				[
					{
						"distribution": "fedora-30",
						"architecture": "x86_64",
						"image_type": "invalid",
						"repositories":
						[
							{
								"baseurl": "http://mirrors.kernel.org/fedora/releases/30/Everything/x86_64/os/"
							}
						]
					}
				]
			}`,
			"application/json",
			http.StatusBadRequest,
		},
	}

	for n, c := range cases {
		// Submit job
		t.Logf("RCM API submit compose, case %d\n", n)
		resp := internalRequest(api, c.Method, c.Path, c.Body, c.ContentType)
		if resp.StatusCode != c.ExpectedStatus {
			errReason, _ := ioutil.ReadAll(resp.Body)
			t.Fatal("Failed to call /v1/compose, HTTP status code: ", resp.StatusCode, "Error message: ", string(errReason))
		}
		if resp.StatusCode == http.StatusOK {
			decoder := json.NewDecoder(resp.Body)
			decoder.DisallowUnknownFields()
			err := decoder.Decode(&submit_reply)
			if err != nil {
				t.Fatal("Failed to decode response to /v1/compose:", err)
			}
		}
	}
}

func TestStatus(t *testing.T) {
	// Test the most basic use case: Submit a new job and get its status.
	registry, err := distro_mock.NewDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}

	workers, dir := newTestWorkerServer(t)
	defer cleanupTempDir(t, dir)

	api := rcm.New(nil, workers, rpmmd_mock.NewRPMMDMock(rpmmd_mock.BaseFixture()), registry)

	var submit_reply struct {
		UUID uuid.UUID `json:"compose_id"`
	}
	var status_reply struct {
		Status      string `json:"status,omitempty"`
		ErrorReason string `json:"error_reason,omitempty"`
	}
	// Submit a job
	resp := internalRequest(api,
		"POST",
		"/v1/compose",
		`{
				"image_builds":
				[
					{
						"distribution": "fedora-30",
						"architecture": "x86_64",
						"image_type": "qcow2",
						"repositories":
						[
							{
								"baseurl": "http://mirrors.kernel.org/fedora/releases/30/Everything/x86_64/os/"
							}
						]
					}
				]
			}`,
		"application/json")
	if resp.StatusCode != http.StatusOK {
		errReason, _ := ioutil.ReadAll(resp.Body)
		t.Fatal("Failed to call /v1/compose, HTTP status code: ", resp.StatusCode, "Error message: ", string(errReason))
	}
	decoder := json.NewDecoder(resp.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&submit_reply)
	if err != nil {
		t.Fatal("Failed to decode response to /v1/compose:", err)
	}

	var cases = []struct {
		Method         string
		Path           string
		Body           string
		ContentType    string
		ExpectedStatus string
	}{
		{
			"GET",
			"/v1/compose/" + submit_reply.UUID.String(),
			``,
			"application/json",
			"WAITING",
		},
	}

	for n, c := range cases {
		// Get the status
		t.Logf("RCM API get compose status, case %d\n", n)
		resp = internalRequest(api, c.Method, c.Path, c.Body, c.ContentType)
		decoder = json.NewDecoder(resp.Body)
		decoder.DisallowUnknownFields()
		err = decoder.Decode(&status_reply)
		if err != nil {
			t.Fatal("Failed to decode response to /v1/compose/UUID:", err)
		}
		if status_reply.Status != c.ExpectedStatus {
			t.Error("Failed to get compose status:", status_reply.Status)
		}
	}
}
