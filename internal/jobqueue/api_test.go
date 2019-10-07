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

func sendHTTP(api *jobqueue.API, method, path, body string) {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	api.ServeHTTP(resp, req)
}

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
		api := jobqueue.New(nil, store.New(nil))

		testRoute(t, api, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
	}
}

func TestCreate(t *testing.T) {
	id, _ := uuid.Parse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	store := store.New(nil)
	api := jobqueue.New(nil, store)

	store.PushCompose(id, &blueprint.Blueprint{}, "tar")

	testRoute(t, api, "POST", "/job-queue/v1/jobs", `{}`, http.StatusCreated,
		`{"id":"ffffffff-ffff-ffff-ffff-ffffffffffff","pipeline":{"build":{"stages":[{"name":"org.osbuild.dnf","options":{"repos":[{"metalink":"https://mirrors.fedoraproject.org/metalink?repo=fedora-$releasever\u0026arch=$basearch","gpgkey":"F1D8 EC98 F241 AAF2 0DF6  9420 EF3C 111F CFC6 59B9","checksum":"sha256:9f596e18f585bee30ac41c11fb11a83ed6b11d5b341c1cb56ca4015d7717cb97"}],"packages":["dnf","e2fsprogs","policycoreutils","qemu-img","systemd","grub2-pc","tar"],"releasever":"30","basearch":"x86_64"}}]},"stages":[{"name":"org.osbuild.dnf","options":{"repos":[{"metalink":"https://mirrors.fedoraproject.org/metalink?repo=fedora-$releasever\u0026arch=$basearch","gpgkey":"F1D8 EC98 F241 AAF2 0DF6  9420 EF3C 111F CFC6 59B9","checksum":"sha256:9f596e18f585bee30ac41c11fb11a83ed6b11d5b341c1cb56ca4015d7717cb97"}],"packages":["@Core","chrony","kernel","selinux-policy-targeted","grub2-pc","spice-vdagent","qemu-guest-agent","xen-libs","langpacks-en"],"releasever":"30","basearch":"x86_64"}},{"name":"org.osbuild.fix-bls","options":{}},{"name":"org.osbuild.locale","options":{"language":"en_US"}},{"name":"org.osbuild.selinux","options":{"file_contexts":"etc/selinux/targeted/contexts/files/file_contexts"}}],"assembler":{"name":"org.osbuild.tar","options":{"filename":"image.tar"}}},"targets":[{"name":"org.osbuild.local","options":{"location":"/var/lib/osbuild-composer/outputs/ffffffff-ffff-ffff-ffff-ffffffffffff"}}]}`)
}

func testUpdateTransition(t *testing.T, from, to string, expectedStatus int) {
	id, _ := uuid.Parse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	store := store.New(nil)
	api := jobqueue.New(nil, store)

	store.PushCompose(id, &blueprint.Blueprint{}, "tar")

	sendHTTP(api, "POST", "/job-queue/v1/jobs", `{}`)
	if from != "WAITING" {
		sendHTTP(api, "PATCH", "/job-queue/v1/jobs/ffffffff-ffff-ffff-ffff-ffffffffffff", `{"status":"`+from+`"}`)
	}
	testRoute(t, api, "PATCH", "/job-queue/v1/jobs/ffffffff-ffff-ffff-ffff-ffffffffffff", `{"status":"`+to+`"}`, expectedStatus, ``)
}

func TestUpdate(t *testing.T) {
	var cases = []struct {
		From           string
		To             string
		ExpectedStatus int
	}{
		{"WAITING", "WAITING", http.StatusBadRequest},
		{"WAITING", "RUNNING", http.StatusOK},
		{"WAITING", "FINISHED", http.StatusOK},
		{"WAITING", "FAILED", http.StatusOK},
		{"RUNNING", "RUNNING", http.StatusOK},
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
