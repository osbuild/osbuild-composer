package weldr_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/weldr"
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

func externalRequest(method, path, body string) *http.Response {
	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", "/run/weldr/api.socket")
			},
		},
	}

	req, err := http.NewRequest(method, "http://localhost"+path, bytes.NewReader([]byte(body)))
	if err != nil {
		panic(err)
	}

	if method == "POST" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	return resp
}

func internalRequest(api *weldr.API, method, path, body string) *http.Response {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))

	if method == "POST" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp := httptest.NewRecorder()
	api.ServeHTTP(resp, req)

	return resp.Result()
}

func sendHTTP(api *weldr.API, external bool, method, path, body string) *http.Response {
	if len(os.Getenv("OSBUILD_COMPOSER_TEST_EXTERNAL")) > 0 {
		if !external {
			return nil
		}
		return externalRequest(method, path, body)
	} else {
		return internalRequest(api, method, path, body)
	}
}

// this function serves to drop fields that shouldn't be tested from the unmarshalled json objects
func dropFields(obj interface{}, fields ...string) {
	switch v := obj.(type) {
	// if the interface type is a map attempt to delete the fields
	case map[string]interface{}:
		for i, field := range fields {
			if _, ok := v[field]; ok {
				delete(v, field)
				// if the field is found remove it from the fields slice
				if len(fields) < i-1 {
					fields = append(fields[:i], fields[i+1:]...)
				} else {
					fields = fields[:i]
				}
			}
		}
		// call dropFields on the remaining elements since they may contain a map containing the field
		for _, val := range v {
			dropFields(val, fields...)
		}
	// if the type is a list of interfaces call dropFields on each interface
	case []interface{}:
		for _, element := range v {
			dropFields(element, fields...)
		}
	default:
		return
	}
}

func testRoute(t *testing.T, api *weldr.API, external bool, method, path, body string, expectedStatus int, expectedJSON string, ignoreFields ...string) {
	resp := sendHTTP(api, external, method, path, body)
	if resp == nil {
		t.Skip("This test is for internal testing only")
	}

	if resp.StatusCode != expectedStatus {
		t.Errorf("%s: expected status %v, but got %v", path, expectedStatus, resp.StatusCode)
		return
	}

	replyJSON, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("%s: could not read response body: %v", path, err)
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

	dropFields(reply, ignoreFields...)
	dropFields(expected, ignoreFields...)

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
		{"/api/v0/projects/source/info/", http.StatusNotFound, `{"errors":[{"code":404,"id":"HTTPError","msg":"Not Found"}],"status":false}`},
		{"/api/v0/projects/source/info/foo", http.StatusOK, `{"errors":[{"id":"UnknownSource","msg":"foo is not a valid source"}],"sources":{}}`},
		{"/api/v0/projects/source/info/test", http.StatusOK, `{"sources":{"Test":{"name":"Test","type":"yum-baseurl","url":"http://example.com/test/os","check_gpg":true,"check_ssl":true,"system":true}},"errors":[]}`},
		{"/api/v0/projects/source/info/*", http.StatusOK, `{"sources":{"Test":{"name":"Test","type":"yum-baseurl","url":"http://example.com/test/os","check_gpg":true,"check_ssl":true,"system":true}},"errors":[]}`},

		{"/api/v0/modules/list", http.StatusOK, `{"total":2,"offset":0,"limit":20,"modules":[{"name":"package1","group_type":"rpm"},{"name":"package2","group_type":"rpm"}]}`},
		{"/api/v0/modules/list/*", http.StatusOK, `{"total":2,"offset":0,"limit":20,"modules":[{"name":"package1","group_type":"rpm"},{"name":"package2","group_type":"rpm"}]}`},
		{"/api/v0/modules/list?offset=1", http.StatusOK, `{"total":2,"offset":1,"limit":20,"modules":[{"name":"package2","group_type":"rpm"}]}`},
		{"/api/v0/modules/list?limit=1", http.StatusOK, `{"total":2,"offset":0,"limit":1,"modules":[{"name":"package1","group_type":"rpm"}]}`},
		{"/api/v0/modules/list?limit=0", http.StatusOK, `{"total":2,"offset":0,"limit":0,"modules":[]}`},
		{"/api/v0/modules/list?offset=10&limit=10", http.StatusOK, `{"total":2,"offset":10,"limit":10,"modules":[]}`},
		{"/api/v0/modules/list/foo", http.StatusBadRequest, `{"errors":[{"id":"UnknownModule","msg":"one of the requested modules does not exist: ['foo']"}],"status":false}`}, // returns empty list instead of an error for unknown packages
		{"/api/v0/modules/list/package2", http.StatusOK, `{"total":1,"offset":0,"limit":20,"modules":[{"name":"package2","group_type":"rpm"}]}`},
		{"/api/v0/modules/list/*package2*", http.StatusOK, `{"total":1,"offset":0,"limit":20,"modules":[{"name":"package2","group_type":"rpm"}]}`},
		{"/api/v0/modules/list/*package*", http.StatusOK, `{"total":2,"offset":0,"limit":20,"modules":[{"name":"package1","group_type":"rpm"},{"name":"package2","group_type":"rpm"}]}`},

		{"/api/v0/modules/info", http.StatusNotFound, ``},
		{"/api/v0/modules/info/", http.StatusNotFound, `{"errors":[{"code":404,"id":"HTTPError","msg":"Not Found"}],"status":false}`},

		{"/api/v0/blueprints/list", http.StatusOK, `{"total":0,"offset":0,"limit":0,"blueprints":[]}`},
		{"/api/v0/blueprints/info/", http.StatusNotFound, `{"errors":[{"code":404,"id":"HTTPError","msg":"Not Found"}],"status":false}`},
		{"/api/v0/blueprints/info/foo", http.StatusBadRequest, `{"errors":[{"id":"UnknownBlueprint","msg":"foo: "}],"status":false}`},
	}

	for _, c := range cases {
		api := weldr.New(repo, packages, nil, store.New(nil))
		testRoute(t, api, true, "GET", c.Path, ``, c.ExpectedStatus, c.ExpectedJSON)
	}
}

func TestBlueprintsNew(t *testing.T) {
	var cases = []struct {
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{"POST", "/api/v0/blueprints/new", `{"name":"test","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`, http.StatusOK, `{"status":true}`},
	}

	for _, c := range cases {
		api := weldr.New(repo, packages, nil, store.New(nil))
		testRoute(t, api, true, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
	}
}

func TestBlueprintsWorkspace(t *testing.T) {
	var cases = []struct {
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{"POST", "/api/v0/blueprints/workspace", `{"name":"test","description":"Test","packages":[{"name":"systemd","version":"123"}],"version":"0.0.0"}`, http.StatusOK, `{"status":true}`},
	}

	for _, c := range cases {
		api := weldr.New(repo, packages, nil, store.New(nil))
		sendHTTP(api, true, "POST", "/api/v0/blueprints/new", `{"name":"test","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
		testRoute(t, api, true, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
	}
}

func TestBlueprintsInfo(t *testing.T) {
	var cases = []struct {
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{"GET", "/api/v0/blueprints/info/test1", ``, http.StatusOK, `{"blueprints":[{"name":"test1","description":"Test","modules":[],"packages":[{"name":"httpd","version":"2.4.*"}],"groups":[],"version":"0.0.0"}],
		"changes":[{"name":"test1","changed":false}], "errors":[]}`},
		{"GET", "/api/v0/blueprints/info/test2", ``, http.StatusOK, `{"blueprints":[{"name":"test2","description":"Test","modules":[],"packages":[{"name":"systemd","version":"123"}],"groups":[],"version":"0.0.0"}],
		"changes":[{"name":"test2","changed":true}], "errors":[]}`},
	}

	for _, c := range cases {
		api := weldr.New(repo, packages, nil, store.New(nil))
		sendHTTP(api, true, "POST", "/api/v0/blueprints/new", `{"name":"test1","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
		sendHTTP(api, true, "POST", "/api/v0/blueprints/new", `{"name":"test2","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
		sendHTTP(api, true, "POST", "/api/v0/blueprints/workspace", `{"name":"test2","description":"Test","packages":[{"name":"systemd","version":"123"}],"version":"0.0.0"}`)
		testRoute(t, api, true, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
		sendHTTP(api, true, "DELETE", "/api/v0/blueprints/delete/test2", ``)
		sendHTTP(api, true, "DELETE", "/api/v0/blueprints/delete/test1", ``)
	}
}

func TestBlueprintsDiff(t *testing.T) {
	var cases = []struct {
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{"GET", "/api/v0/blueprints/diff/test/NEWEST/WORKSPACE", ``, http.StatusOK, `{"diff":[{"new":{"Package":{"name":"systemd","version":"123"}},"old":null},{"new":null,"old":{"Package":{"name":"httpd","version":"2.4.*"}}}]}`},
	}

	for _, c := range cases {
		api := weldr.New(repo, packages, nil, store.New(nil))
		sendHTTP(api, true, "POST", "/api/v0/blueprints/new", `{"name":"test","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
		sendHTTP(api, true, "POST", "/api/v0/blueprints/workspace", `{"name":"test","description":"Test","packages":[{"name":"systemd","version":"123"}],"version":"0.0.0"}`)
		testRoute(t, api, true, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
		sendHTTP(api, true, "DELETE", "/api/v0/blueprints/delete/test", ``)
	}
}

func TestBlueprintsDelete(t *testing.T) {
	var cases = []struct {
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{"DELETE", "/api/v0/blueprints/delete/test", ``, http.StatusOK, `{"status":true}`},
	}

	for _, c := range cases {
		api := weldr.New(repo, packages, nil, store.New(nil))
		sendHTTP(api, true, "POST", "/api/v0/blueprints/new", `{"name":"test","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
		testRoute(t, api, true, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
		sendHTTP(api, true, "DELETE", "/api/v0/blueprints/delete/test", ``)
	}
}

func TestCompose(t *testing.T) {
	var cases = []struct {
		External       bool
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
		ExpectedJSON   string
		IgnoreFields   []string
	}{
		{true, "POST", "/api/v0/compose", `{"blueprint_name": "http-server","compose_type": "tar","branch": "master"}`, http.StatusBadRequest, `{"status":false,"errors":[{"id":"UnknownBlueprint","msg":"Unknown blueprint name: http-server"}]}`, []string{"build_id"}},
		{false, "POST", "/api/v0/compose", `{"blueprint_name": "test","compose_type": "tar","branch": "master"}`, http.StatusOK, `{"status": true}`, []string{"build_id"}},
	}

	for _, c := range cases {
		api := weldr.New(repo, packages, nil, store.New(nil))
		sendHTTP(api, c.External, "POST", "/api/v0/blueprints/new", `{"name":"test","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
		testRoute(t, api, c.External, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON, c.IgnoreFields...)
		sendHTTP(api, c.External, "DELETE", "/api/v0/blueprints/delete/test", ``)
	}
}

func TestComposeQueue(t *testing.T) {
	var cases = []struct {
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
		ExpectedJSON   string
		IgnoreFields   []string
	}{
		{"GET", "/api/v0/compose/queue", ``, http.StatusOK, `{"new":[{"blueprint":"test","version":"0.0.0","compose_type":"tar","image_size":0,"queue_status":"WAITING"}],"run":[{"blueprint":"test","version":"0.0.0","compose_type":"tar","image_size":0,"queue_status":"RUNNING"}]}`, []string{"id", "job_created", "job_started"}},
	}

	if len(os.Getenv("OSBUILD_COMPOSER_TEST_EXTERNAL")) > 0 {
		t.Skip("This test is for internal testing only")
	}

	for _, c := range cases {
		s := store.New(nil)
		api := weldr.New(repo, packages, nil, s)
		sendHTTP(api, false, "POST", "/api/v0/blueprints/new", `{"name":"test","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
		// create job and leave it waiting
		sendHTTP(api, false, "POST", "/api/v0/compose", `{"blueprint_name": "test","compose_type": "tar","branch": "master"}`)
		// create job and leave it running
		sendHTTP(api, false, "POST", "/api/v0/compose", `{"blueprint_name": "test","compose_type": "tar","branch": "master"}`)
		s.PopCompose()
		// create job and mark it as finished
		sendHTTP(api, false, "POST", "/api/v0/compose", `{"blueprint_name": "test","compose_type": "tar","branch": "master"}`)
		job := s.PopCompose()
		s.UpdateCompose(job.ComposeID, "FINISHED")
		// create job and mark it as failed
		sendHTTP(api, false, "POST", "/api/v0/compose", `{"blueprint_name": "test","compose_type": "tar","branch": "master"}`)
		job = s.PopCompose()
		s.UpdateCompose(job.ComposeID, "FAILED")

		testRoute(t, api, false, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON, c.IgnoreFields...)
		sendHTTP(api, false, "DELETE", "/api/v0/blueprints/delete/test", ``)
	}
}

func TestSourcesNew(t *testing.T) {
	var cases = []struct {
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{"POST", "/api/v0/projects/source/new", ``, http.StatusBadRequest, `{"errors":[{"code":400,"id":"HTTPError","msg":"Bad Request"}],"status":false}`},
		{"POST", "/api/v0/projects/source/new", `{"name": "fish","url": "https://download.opensuse.org/repositories/shells:/fish:/release:/3/Fedora_29/","type": "yum-baseurl","check_ssl": false,"check_gpg": false}`, http.StatusOK, `{"status":true}`},
	}

	for _, c := range cases {
		api := weldr.New(repo, packages, nil, store.New(nil))
		testRoute(t, api, true, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
		sendHTTP(api, true, "DELETE", "/api/v0/projects/source/delete/fish", ``)
	}
}

func TestSourcesDelete(t *testing.T) {
	var cases = []struct {
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{"DELETE", "/api/v0/projects/source/delete/", ``, http.StatusNotFound, `{"status":false,"errors":[{"code":404,"id":"HTTPError","msg":"Not Found"}]}`},
		{"DELETE", "/api/v0/projects/source/delete/fish", ``, http.StatusOK, `{"status":true}`},
	}

	for _, c := range cases {
		api := weldr.New(repo, packages, nil, store.New(nil))
		sendHTTP(api, true, "POST", "/api/v0/projects/source/new", `{"name": "fish","url": "https://download.opensuse.org/repositories/shells:/fish:/release:/3/Fedora_29/","type": "yum-baseurl","check_ssl": false,"check_gpg": false}`)
		testRoute(t, api, true, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
		sendHTTP(api, true, "DELETE", "/api/v0/projects/source/delete/fish", ``)
	}
}
