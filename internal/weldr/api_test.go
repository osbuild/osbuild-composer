package weldr_test

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
	"osbuild-composer/internal/rpmmd"
	"osbuild-composer/internal/weldr"
)

var repo = rpmmd.RepoConfig{
	Id:      "test",
	Name:    "Test",
	BaseURL: "http://example.com/test/os",
}

var packages = rpmmd.PackageList{
	{Name: "package1"},
	{Name: "package2"},
}

func testRoute(t *testing.T, api *weldr.API, method, path, body string, expectedStatus int, expectedJSON string) {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	if method == "POST" {
		req.Header.Set("Content-Type", "application/json")
	}
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
		Path           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{"/api/status", http.StatusOK, `{"api":1,"db_supported":true,"db_version":"0","schema_version":"0","backend":"osbuild-composer","build":"devel","messages":[]}`},

		{"/api/v0/projects/source/list", http.StatusOK, `{"sources":["test"]}`},

		{"/api/v0/projects/source/info", http.StatusNotFound, ``},
		{"/api/v0/projects/source/info/", http.StatusNotFound, ``},
		{"/api/v0/projects/source/info/foo", http.StatusBadRequest, `{"status":false,"errors":["repository not found: foo"]}`},
		{"/api/v0/projects/source/info/test", http.StatusOK, `{"sources":{"test":{"id":"test","name":"Test","type":"yum-baseurl","url":"http://example.com/test/os","check_gpg":true,"check_ssl":true,"system":true}}}`},
		{"/api/v0/projects/source/info/*", http.StatusOK, `{"sources":{"test":{"id":"test","name":"Test","type":"yum-baseurl","url":"http://example.com/test/os","check_gpg":true,"check_ssl":true,"system":true}}}`},

		{"/api/v0/modules/list", http.StatusOK, `{"total":2,"offset":0,"limit":20,"modules":[{"name":"package1","group_type":"rpm"},{"name":"package2","group_type":"rpm"}]}`},
		{"/api/v0/modules/list/*", http.StatusOK, `{"total":2,"offset":0,"limit":20,"modules":[{"name":"package1","group_type":"rpm"},{"name":"package2","group_type":"rpm"}]}`},
		{"/api/v0/modules/list?offset=1", http.StatusOK, `{"total":2,"offset":1,"limit":20,"modules":[{"name":"package2","group_type":"rpm"}]}`},
		{"/api/v0/modules/list?limit=1", http.StatusOK, `{"total":2,"offset":0,"limit":1,"modules":[{"name":"package1","group_type":"rpm"}]}`},
		{"/api/v0/modules/list?limit=0", http.StatusOK, `{"total":2,"offset":0,"limit":0,"modules":[]}`},
		{"/api/v0/modules/list?offset=10&limit=10", http.StatusOK, `{"total":2,"offset":10,"limit":10,"modules":[]}`},
		{"/api/v0/modules/list/foo", http.StatusOK, `{"total":0,"offset":0,"limit":20,"modules":[]}`}, // returns empty list instead of an error for unknown packages
		{"/api/v0/modules/list/package2", http.StatusOK, `{"total":1,"offset":0,"limit":20,"modules":[{"name":"package2","group_type":"rpm"}]}`},
		{"/api/v0/modules/list/*package2*", http.StatusOK, `{"total":1,"offset":0,"limit":20,"modules":[{"name":"package2","group_type":"rpm"}]}`},
		{"/api/v0/modules/list/*package*", http.StatusOK, `{"total":2,"offset":0,"limit":20,"modules":[{"name":"package1","group_type":"rpm"},{"name":"package2","group_type":"rpm"}]}`},

		{"/api/v0/modules/info", http.StatusNotFound, ``},
		{"/api/v0/modules/info/", http.StatusNotFound, ``},

		{"/api/v0/blueprints/list", http.StatusOK, `{"total":1,"offset":0,"limit":1,"blueprints":["example"]}`},
		{"/api/v0/blueprints/info/", http.StatusNotFound, ``},
		{"/api/v0/blueprints/info/foo", http.StatusNotFound, `{"status":false}`},
		{"/api/v0/blueprints/info/example", http.StatusOK, `*`},
	}

	for _, c := range cases {
		api := weldr.New(repo, packages, nil, nil, nil, nil, nil)
		testRoute(t, api, "GET", c.Path, ``, c.ExpectedStatus, c.ExpectedJSON)
	}
}

func TestBlueprints(t *testing.T) {
	api := weldr.New(repo, packages, nil, nil, nil, nil, nil)

	testRoute(t, api, "POST", "/api/v0/blueprints/new",
		`{"name":"test","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0"}`,
		http.StatusOK, `{"status":true}`)

	testRoute(t, api, "GET", "/api/v0/blueprints/info/test", ``,
		http.StatusOK, `{"blueprints":[{"name":"test","description":"Test","modules":[],"packages":[{"name":"httpd","version":"2.4.*"}],"version":"0"}],
		"changes":[{"name":"test","changed":false}], "errors":[]}`)

	testRoute(t, api, "POST", "/api/v0/blueprints/workspace",
		`{"name":"test","description":"Test","packages":[{"name":"systemd","version":"123"}],"version":"0"}`,
		http.StatusOK, `{"status":true}`)

	testRoute(t, api, "GET", "/api/v0/blueprints/info/test", ``,
		http.StatusOK, `{"blueprints":[{"name":"test","description":"Test","modules":[],"packages":[{"name":"systemd","version":"123"}],"version":"0"}],
		"changes":[{"name":"test","changed":true}], "errors":[]}`)

	testRoute(t, api, "GET", "/api/v0/blueprints/diff/test/NEWEST/WORKSPACE", ``,
		http.StatusOK, `{"diff":[{"new":{"Package":{"name":"systemd","version":"123"}},"old":null},{"new":null,"old":{"Package":{"name":"httpd","version":"2.4.*"}}}]}`)
}

func TestCompose(t *testing.T) {
	jobChannel := make(chan job.Job, 200)
	api := weldr.New(repo, packages, nil, nil, nil, jobChannel, nil)

	testRoute(t, api, "POST", "/api/v0/blueprints/new",
		`{"name":"test","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0"}`,
		http.StatusOK, `{"status":true}`)

	testRoute(t, api, "POST", "/api/v0/compose", `{"blueprint_name": "http-server","compose_type": "tar","branch": "master"}`,
		http.StatusBadRequest, `{"status":false,"errors":["blueprint does not exist"]}`)

	testRoute(t, api, "POST", "/api/v0/compose", `{"blueprint_name": "test","compose_type": "tar","branch": "master"}`,
		http.StatusOK, `*`)
}
