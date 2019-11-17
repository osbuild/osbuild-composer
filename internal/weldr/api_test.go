package weldr_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	rpmmd_mock "github.com/osbuild/osbuild-composer/internal/mocks/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/weldr"
)

var repo = rpmmd.RepoConfig{
	Id:      "test",
	Name:    "Test",
	BaseURL: "http://example.com/test/os",
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

func createWeldrAPI(fixture rpmmd_mock.Fixture) (*weldr.API, *store.Store) {
	s := store.New(nil)
	rpm := rpmmd_mock.NewRPMMDMock(fixture)

	return weldr.New(rpm, repo, nil, s), s
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
		{"/api/v0/projects/source/info/test", http.StatusOK, `{"sources":{"test":{"name":"test","type":"yum-baseurl","url":"http://example.com/test/os","check_gpg":true,"check_ssl":true,"system":true}},"errors":[]}`},
		{"/api/v0/projects/source/info/*", http.StatusOK, `{"sources":{"test":{"name":"test","type":"yum-baseurl","url":"http://example.com/test/os","check_gpg":true,"check_ssl":true,"system":true}},"errors":[]}`},

		{"/api/v0/blueprints/list", http.StatusOK, `{"total":0,"offset":0,"limit":0,"blueprints":[]}`},
		{"/api/v0/blueprints/info/", http.StatusNotFound, `{"errors":[{"code":404,"id":"HTTPError","msg":"Not Found"}],"status":false}`},
		{"/api/v0/blueprints/info/foo", http.StatusBadRequest, `{"errors":[{"id":"UnknownBlueprint","msg":"foo: "}],"status":false}`},
	}

	for _, c := range cases {
		api, _ := createWeldrAPI(rpmmd_mock.BaseFixture)
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
		api, _ := createWeldrAPI(rpmmd_mock.BaseFixture)
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
		api, _ := createWeldrAPI(rpmmd_mock.BaseFixture)
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
		api, _ := createWeldrAPI(rpmmd_mock.BaseFixture)
		sendHTTP(api, true, "POST", "/api/v0/blueprints/new", `{"name":"test1","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
		sendHTTP(api, true, "POST", "/api/v0/blueprints/new", `{"name":"test2","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
		sendHTTP(api, true, "POST", "/api/v0/blueprints/workspace", `{"name":"test2","description":"Test","packages":[{"name":"systemd","version":"123"}],"version":"0.0.0"}`)
		testRoute(t, api, true, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
		sendHTTP(api, true, "DELETE", "/api/v0/blueprints/delete/test2", ``)
		sendHTTP(api, true, "DELETE", "/api/v0/blueprints/delete/test1", ``)
	}
}

func TestBlueprintsFreeze(t *testing.T) {
	var cases = []struct {
		Fixture        rpmmd_mock.Fixture
		Path           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{rpmmd_mock.BaseFixture, "/api/v0/blueprints/freeze/test", http.StatusOK, `{"blueprints":[{"blueprint":{"name":"test","description":"Test","version":"0.0.0","packages":[{"name":"dep-package1","version":"1.33-2.fc30.x86_64"}],"modules":[],"groups":[]}}],"errors":[]}`},
	}

	for _, c := range cases {
		api, _ := createWeldrAPI(c.Fixture)
		sendHTTP(api, false, "POST", "/api/v0/blueprints/new", `{"name":"test","description":"Test","packages":[{"name":"dep-package1","version":"*"}],"version":"0.0.0"}`)
		testRoute(t, api, false, "GET", c.Path, ``, c.ExpectedStatus, c.ExpectedJSON)
		sendHTTP(api, false, "DELETE", "/api/v0/blueprints/delete/test", ``)
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
		api, _ := createWeldrAPI(rpmmd_mock.BaseFixture)
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
		api, _ := createWeldrAPI(rpmmd_mock.BaseFixture)
		sendHTTP(api, true, "POST", "/api/v0/blueprints/new", `{"name":"test","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
		testRoute(t, api, true, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
		sendHTTP(api, true, "DELETE", "/api/v0/blueprints/delete/test", ``)
	}
}

func TestBlueprintsChanges(t *testing.T) {
	api, _ := createWeldrAPI(rpmmd_mock.BaseFixture)
	rand.Seed(time.Now().UnixNano())
	id := strconv.Itoa(rand.Int())
	ignoreFields := []string{"commit", "timestamp"}

	sendHTTP(api, true, "POST", "/api/v0/blueprints/new", `{"name":"`+id+`","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
	testRoute(t, api, true, "GET", "/api/v0/blueprints/changes/failing"+id, ``, http.StatusOK, `{"blueprints":[],"errors":[{"id":"UnknownBlueprint","msg":"failing`+id+`"}],"limit":20,"offset":0}`, ignoreFields...)
	testRoute(t, api, true, "GET", "/api/v0/blueprints/changes/"+id, ``, http.StatusOK, `{"blueprints":[{"changes":[{"commit":"","message":"Recipe `+id+`, version 0.0.0 saved.","revision":null,"timestamp":""}],"name":"`+id+`","total":1}],"errors":[],"limit":20,"offset":0}`, ignoreFields...)
	sendHTTP(api, true, "DELETE", "/api/v0/blueprints/delete/"+id, ``)
	sendHTTP(api, true, "POST", "/api/v0/blueprints/new", `{"name":"`+id+`","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
	testRoute(t, api, true, "GET", "/api/v0/blueprints/changes/"+id, ``, http.StatusOK, `{"blueprints":[{"changes":[{"commit":"","message":"Recipe `+id+`, version 0.0.0 saved.","revision":null,"timestamp":""},{"commit":"","message":"Recipe `+id+`, version 0.0.0 saved.","revision":null,"timestamp":""}],"name":"`+id+`","total":2}],"errors":[],"limit":20,"offset":0}`, ignoreFields...)
	sendHTTP(api, true, "DELETE", "/api/v0/blueprints/delete/"+id, ``)
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
		api, _ := createWeldrAPI(rpmmd_mock.BaseFixture)
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
		api, s := createWeldrAPI(rpmmd_mock.BaseFixture)
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
		api, _ := createWeldrAPI(rpmmd_mock.BaseFixture)
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
		api, _ := createWeldrAPI(rpmmd_mock.BaseFixture)
		sendHTTP(api, true, "POST", "/api/v0/projects/source/new", `{"name": "fish","url": "https://download.opensuse.org/repositories/shells:/fish:/release:/3/Fedora_29/","type": "yum-baseurl","check_ssl": false,"check_gpg": false}`)
		testRoute(t, api, true, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
		sendHTTP(api, true, "DELETE", "/api/v0/projects/source/delete/fish", ``)
	}
}

func TestProjectsDepsolve(t *testing.T) {
	var cases = []struct {
		Fixture        rpmmd_mock.Fixture
		Path           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{rpmmd_mock.NonExistingPackage, "/api/v0/projects/depsolve/fash", http.StatusBadRequest, `{"status":false,"errors":[{"id":"PROJECTS_ERROR","msg":"BadRequest: DNF error occured: MarkingErrors: Error occurred when marking packages for installation: Problems in request:\nmissing packages: fash"}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/projects/depsolve/fish", http.StatusOK, `{"projects":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]}`},
		{rpmmd_mock.BadDepsolve, "/api/v0/projects/depsolve/go2rpm", http.StatusBadRequest, `{"status":false,"errors":[{"id":"PROJECTS_ERROR","msg":"BadRequest: DNF error occured: DepsolveError: There was a problem depsolving ['go2rpm']: \n Problem: conflicting requests\n  - nothing provides askalono-cli needed by go2rpm-1-4.fc31.noarch"}]}`},
	}

	for _, c := range cases {
		api, _ := createWeldrAPI(c.Fixture)
		testRoute(t, api, true, "GET", c.Path, ``, c.ExpectedStatus, c.ExpectedJSON)
	}
}

func TestProjectsInfo(t *testing.T) {
	var cases = []struct {
		Fixture        rpmmd_mock.Fixture
		Path           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{rpmmd_mock.BaseFixture, "/api/v0/projects/info", http.StatusBadRequest, `{"status":false,"errors":[{"id":"UnknownProject","msg":"No packages specified."}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/projects/info/", http.StatusBadRequest, `{"status":false,"errors":[{"id":"UnknownProject","msg":"No packages specified."}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/projects/info/nonexistingpkg", http.StatusBadRequest, `{"status":false,"errors":[{"id":"UnknownProject","msg":"No packages have been found."}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/projects/info/*", http.StatusOK, `{"projects":[{"name":"package0","summary":"pkg0 sum","description":"pkg0 desc","homepage":"https://pkg0.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-01-03T15:04:05Z","epoch":0,"release":"0.fc30","source":{"license":"MIT","version":"0.1"}},{"arch":"x86_64","build_time":"2006-01-02T15:04:05Z","epoch":0,"release":"0.fc30","source":{"license":"MIT","version":"0.0"}}]},{"name":"package1","summary":"pkg1 sum","description":"pkg1 desc","homepage":"https://pkg1.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-02-02T15:04:05Z","epoch":0,"release":"1.fc30","source":{"license":"MIT","version":"1.0"}},{"arch":"x86_64","build_time":"2006-02-03T15:04:05Z","epoch":0,"release":"1.fc30","source":{"license":"MIT","version":"1.1"}}]},{"name":"package10","summary":"pkg10 sum","description":"pkg10 desc","homepage":"https://pkg10.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-11-02T15:04:05Z","epoch":0,"release":"10.fc30","source":{"license":"MIT","version":"10.0"}},{"arch":"x86_64","build_time":"2006-11-03T15:04:05Z","epoch":0,"release":"10.fc30","source":{"license":"MIT","version":"10.1"}}]},{"name":"package11","summary":"pkg11 sum","description":"pkg11 desc","homepage":"https://pkg11.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-12-03T15:04:05Z","epoch":0,"release":"11.fc30","source":{"license":"MIT","version":"11.1"}},{"arch":"x86_64","build_time":"2006-12-02T15:04:05Z","epoch":0,"release":"11.fc30","source":{"license":"MIT","version":"11.0"}}]},{"name":"package12","summary":"pkg12 sum","description":"pkg12 desc","homepage":"https://pkg12.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-01-02T15:04:05Z","epoch":0,"release":"12.fc30","source":{"license":"MIT","version":"12.0"}},{"arch":"x86_64","build_time":"2007-01-03T15:04:05Z","epoch":0,"release":"12.fc30","source":{"license":"MIT","version":"12.1"}}]},{"name":"package13","summary":"pkg13 sum","description":"pkg13 desc","homepage":"https://pkg13.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-02-02T15:04:05Z","epoch":0,"release":"13.fc30","source":{"license":"MIT","version":"13.0"}},{"arch":"x86_64","build_time":"2007-02-03T15:04:05Z","epoch":0,"release":"13.fc30","source":{"license":"MIT","version":"13.1"}}]},{"name":"package14","summary":"pkg14 sum","description":"pkg14 desc","homepage":"https://pkg14.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-03-03T15:04:05Z","epoch":0,"release":"14.fc30","source":{"license":"MIT","version":"14.1"}},{"arch":"x86_64","build_time":"2007-03-02T15:04:05Z","epoch":0,"release":"14.fc30","source":{"license":"MIT","version":"14.0"}}]},{"name":"package15","summary":"pkg15 sum","description":"pkg15 desc","homepage":"https://pkg15.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-04-03T15:04:05Z","epoch":0,"release":"15.fc30","source":{"license":"MIT","version":"15.1"}},{"arch":"x86_64","build_time":"2007-04-02T15:04:05Z","epoch":0,"release":"15.fc30","source":{"license":"MIT","version":"15.0"}}]},{"name":"package16","summary":"pkg16 sum","description":"pkg16 desc","homepage":"https://pkg16.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-05-02T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.0"}},{"arch":"x86_64","build_time":"2007-05-03T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.1"}}]},{"name":"package17","summary":"pkg17 sum","description":"pkg17 desc","homepage":"https://pkg17.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-06-03T15:04:05Z","epoch":0,"release":"17.fc30","source":{"license":"MIT","version":"17.1"}},{"arch":"x86_64","build_time":"2007-06-02T15:04:05Z","epoch":0,"release":"17.fc30","source":{"license":"MIT","version":"17.0"}}]},{"name":"package18","summary":"pkg18 sum","description":"pkg18 desc","homepage":"https://pkg18.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-07-02T15:04:05Z","epoch":0,"release":"18.fc30","source":{"license":"MIT","version":"18.0"}},{"arch":"x86_64","build_time":"2007-07-03T15:04:05Z","epoch":0,"release":"18.fc30","source":{"license":"MIT","version":"18.1"}}]},{"name":"package19","summary":"pkg19 sum","description":"pkg19 desc","homepage":"https://pkg19.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-08-03T15:04:05Z","epoch":0,"release":"19.fc30","source":{"license":"MIT","version":"19.1"}},{"arch":"x86_64","build_time":"2007-08-02T15:04:05Z","epoch":0,"release":"19.fc30","source":{"license":"MIT","version":"19.0"}}]},{"name":"package2","summary":"pkg2 sum","description":"pkg2 desc","homepage":"https://pkg2.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-03-02T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.0"}},{"arch":"x86_64","build_time":"2006-03-03T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.1"}}]},{"name":"package20","summary":"pkg20 sum","description":"pkg20 desc","homepage":"https://pkg20.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-09-03T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.1"}},{"arch":"x86_64","build_time":"2007-09-02T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.0"}}]},{"name":"package21","summary":"pkg21 sum","description":"pkg21 desc","homepage":"https://pkg21.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-10-02T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.0"}},{"arch":"x86_64","build_time":"2007-10-03T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.1"}}]},{"name":"package3","summary":"pkg3 sum","description":"pkg3 desc","homepage":"https://pkg3.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-04-03T15:04:05Z","epoch":0,"release":"3.fc30","source":{"license":"MIT","version":"3.1"}},{"arch":"x86_64","build_time":"2006-04-02T15:04:05Z","epoch":0,"release":"3.fc30","source":{"license":"MIT","version":"3.0"}}]},{"name":"package4","summary":"pkg4 sum","description":"pkg4 desc","homepage":"https://pkg4.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-05-03T15:04:05Z","epoch":0,"release":"4.fc30","source":{"license":"MIT","version":"4.1"}},{"arch":"x86_64","build_time":"2006-05-02T15:04:05Z","epoch":0,"release":"4.fc30","source":{"license":"MIT","version":"4.0"}}]},{"name":"package5","summary":"pkg5 sum","description":"pkg5 desc","homepage":"https://pkg5.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-06-03T15:04:05Z","epoch":0,"release":"5.fc30","source":{"license":"MIT","version":"5.1"}},{"arch":"x86_64","build_time":"2006-06-02T15:04:05Z","epoch":0,"release":"5.fc30","source":{"license":"MIT","version":"5.0"}}]},{"name":"package6","summary":"pkg6 sum","description":"pkg6 desc","homepage":"https://pkg6.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-07-02T15:04:05Z","epoch":0,"release":"6.fc30","source":{"license":"MIT","version":"6.0"}},{"arch":"x86_64","build_time":"2006-07-03T15:04:05Z","epoch":0,"release":"6.fc30","source":{"license":"MIT","version":"6.1"}}]},{"name":"package7","summary":"pkg7 sum","description":"pkg7 desc","homepage":"https://pkg7.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-08-02T15:04:05Z","epoch":0,"release":"7.fc30","source":{"license":"MIT","version":"7.0"}},{"arch":"x86_64","build_time":"2006-08-03T15:04:05Z","epoch":0,"release":"7.fc30","source":{"license":"MIT","version":"7.1"}}]},{"name":"package8","summary":"pkg8 sum","description":"pkg8 desc","homepage":"https://pkg8.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-09-03T15:04:05Z","epoch":0,"release":"8.fc30","source":{"license":"MIT","version":"8.1"}},{"arch":"x86_64","build_time":"2006-09-02T15:04:05Z","epoch":0,"release":"8.fc30","source":{"license":"MIT","version":"8.0"}}]},{"name":"package9","summary":"pkg9 sum","description":"pkg9 desc","homepage":"https://pkg9.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-10-02T15:04:05Z","epoch":0,"release":"9.fc30","source":{"license":"MIT","version":"9.0"}},{"arch":"x86_64","build_time":"2006-10-03T15:04:05Z","epoch":0,"release":"9.fc30","source":{"license":"MIT","version":"9.1"}}]}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/projects/info/package2*,package16", http.StatusOK, `{"projects":[{"name":"package16","summary":"pkg16 sum","description":"pkg16 desc","homepage":"https://pkg16.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-05-02T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.0"}},{"arch":"x86_64","build_time":"2007-05-03T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.1"}}]},{"name":"package2","summary":"pkg2 sum","description":"pkg2 desc","homepage":"https://pkg2.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-03-03T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.1"}},{"arch":"x86_64","build_time":"2006-03-02T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.0"}}]},{"name":"package20","summary":"pkg20 sum","description":"pkg20 desc","homepage":"https://pkg20.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-09-03T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.1"}},{"arch":"x86_64","build_time":"2007-09-02T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.0"}}]},{"name":"package21","summary":"pkg21 sum","description":"pkg21 desc","homepage":"https://pkg21.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-10-03T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.1"}},{"arch":"x86_64","build_time":"2007-10-02T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.0"}}]}]}`},
		{rpmmd_mock.BadFetch, "/api/v0/projects/info/package2*,package16", http.StatusBadRequest, `{"status":false,"errors":[{"id":"ModulesError","msg":"msg: DNF error occured: FetchError: There was a problem when fetching packages."}]}`},
	}

	for _, c := range cases {
		api, _ := createWeldrAPI(c.Fixture)
		testRoute(t, api, true, "GET", c.Path, ``, c.ExpectedStatus, c.ExpectedJSON)
	}
}

func TestModulesInfo(t *testing.T) {
	var cases = []struct {
		Fixture        rpmmd_mock.Fixture
		Path           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{rpmmd_mock.BaseFixture, "/api/v0/modules/info", http.StatusBadRequest, `{"status":false,"errors":[{"id":"UnknownModule","msg":"No packages specified."}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/modules/info/", http.StatusBadRequest, `{"status":false,"errors":[{"id":"UnknownModule","msg":"No packages specified."}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/modules/info/nonexistingpkg", http.StatusBadRequest, `{"status":false,"errors":[{"id":"UnknownModule","msg":"No packages have been found."}]}`},
		{rpmmd_mock.BadDepsolve, "/api/v0/modules/info/package1", http.StatusBadRequest, `{"status":false,"errors":[{"id":"ModulesError","msg":"Cannot depsolve package package1: DNF error occured: DepsolveError: There was a problem depsolving ['go2rpm']: \n Problem: conflicting requests\n  - nothing provides askalono-cli needed by go2rpm-1-4.fc31.noarch"}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/modules/info/package2*,package16", http.StatusOK, `{"modules":[{"name":"package16","summary":"pkg16 sum","description":"pkg16 desc","homepage":"https://pkg16.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-05-02T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.0"}},{"arch":"x86_64","build_time":"2007-05-03T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package2","summary":"pkg2 sum","description":"pkg2 desc","homepage":"https://pkg2.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-03-02T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.0"}},{"arch":"x86_64","build_time":"2006-03-03T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package20","summary":"pkg20 sum","description":"pkg20 desc","homepage":"https://pkg20.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-09-03T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.1"}},{"arch":"x86_64","build_time":"2007-09-02T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package21","summary":"pkg21 sum","description":"pkg21 desc","homepage":"https://pkg21.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-10-02T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.0"}},{"arch":"x86_64","build_time":"2007-10-03T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/modules/info/*", http.StatusOK, `{"modules":[{"name":"package0","summary":"pkg0 sum","description":"pkg0 desc","homepage":"https://pkg0.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-01-02T15:04:05Z","epoch":0,"release":"0.fc30","source":{"license":"MIT","version":"0.0"}},{"arch":"x86_64","build_time":"2006-01-03T15:04:05Z","epoch":0,"release":"0.fc30","source":{"license":"MIT","version":"0.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package1","summary":"pkg1 sum","description":"pkg1 desc","homepage":"https://pkg1.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-02-02T15:04:05Z","epoch":0,"release":"1.fc30","source":{"license":"MIT","version":"1.0"}},{"arch":"x86_64","build_time":"2006-02-03T15:04:05Z","epoch":0,"release":"1.fc30","source":{"license":"MIT","version":"1.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package10","summary":"pkg10 sum","description":"pkg10 desc","homepage":"https://pkg10.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-11-03T15:04:05Z","epoch":0,"release":"10.fc30","source":{"license":"MIT","version":"10.1"}},{"arch":"x86_64","build_time":"2006-11-02T15:04:05Z","epoch":0,"release":"10.fc30","source":{"license":"MIT","version":"10.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package11","summary":"pkg11 sum","description":"pkg11 desc","homepage":"https://pkg11.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-12-03T15:04:05Z","epoch":0,"release":"11.fc30","source":{"license":"MIT","version":"11.1"}},{"arch":"x86_64","build_time":"2006-12-02T15:04:05Z","epoch":0,"release":"11.fc30","source":{"license":"MIT","version":"11.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package12","summary":"pkg12 sum","description":"pkg12 desc","homepage":"https://pkg12.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-01-02T15:04:05Z","epoch":0,"release":"12.fc30","source":{"license":"MIT","version":"12.0"}},{"arch":"x86_64","build_time":"2007-01-03T15:04:05Z","epoch":0,"release":"12.fc30","source":{"license":"MIT","version":"12.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package13","summary":"pkg13 sum","description":"pkg13 desc","homepage":"https://pkg13.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-02-02T15:04:05Z","epoch":0,"release":"13.fc30","source":{"license":"MIT","version":"13.0"}},{"arch":"x86_64","build_time":"2007-02-03T15:04:05Z","epoch":0,"release":"13.fc30","source":{"license":"MIT","version":"13.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package14","summary":"pkg14 sum","description":"pkg14 desc","homepage":"https://pkg14.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-03-03T15:04:05Z","epoch":0,"release":"14.fc30","source":{"license":"MIT","version":"14.1"}},{"arch":"x86_64","build_time":"2007-03-02T15:04:05Z","epoch":0,"release":"14.fc30","source":{"license":"MIT","version":"14.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package15","summary":"pkg15 sum","description":"pkg15 desc","homepage":"https://pkg15.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-04-03T15:04:05Z","epoch":0,"release":"15.fc30","source":{"license":"MIT","version":"15.1"}},{"arch":"x86_64","build_time":"2007-04-02T15:04:05Z","epoch":0,"release":"15.fc30","source":{"license":"MIT","version":"15.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package16","summary":"pkg16 sum","description":"pkg16 desc","homepage":"https://pkg16.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-05-02T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.0"}},{"arch":"x86_64","build_time":"2007-05-03T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package17","summary":"pkg17 sum","description":"pkg17 desc","homepage":"https://pkg17.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-06-03T15:04:05Z","epoch":0,"release":"17.fc30","source":{"license":"MIT","version":"17.1"}},{"arch":"x86_64","build_time":"2007-06-02T15:04:05Z","epoch":0,"release":"17.fc30","source":{"license":"MIT","version":"17.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package18","summary":"pkg18 sum","description":"pkg18 desc","homepage":"https://pkg18.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-07-02T15:04:05Z","epoch":0,"release":"18.fc30","source":{"license":"MIT","version":"18.0"}},{"arch":"x86_64","build_time":"2007-07-03T15:04:05Z","epoch":0,"release":"18.fc30","source":{"license":"MIT","version":"18.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package19","summary":"pkg19 sum","description":"pkg19 desc","homepage":"https://pkg19.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-08-02T15:04:05Z","epoch":0,"release":"19.fc30","source":{"license":"MIT","version":"19.0"}},{"arch":"x86_64","build_time":"2007-08-03T15:04:05Z","epoch":0,"release":"19.fc30","source":{"license":"MIT","version":"19.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package2","summary":"pkg2 sum","description":"pkg2 desc","homepage":"https://pkg2.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-03-03T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.1"}},{"arch":"x86_64","build_time":"2006-03-02T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package20","summary":"pkg20 sum","description":"pkg20 desc","homepage":"https://pkg20.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-09-03T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.1"}},{"arch":"x86_64","build_time":"2007-09-02T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package21","summary":"pkg21 sum","description":"pkg21 desc","homepage":"https://pkg21.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-10-03T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.1"}},{"arch":"x86_64","build_time":"2007-10-02T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package3","summary":"pkg3 sum","description":"pkg3 desc","homepage":"https://pkg3.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-04-03T15:04:05Z","epoch":0,"release":"3.fc30","source":{"license":"MIT","version":"3.1"}},{"arch":"x86_64","build_time":"2006-04-02T15:04:05Z","epoch":0,"release":"3.fc30","source":{"license":"MIT","version":"3.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package4","summary":"pkg4 sum","description":"pkg4 desc","homepage":"https://pkg4.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-05-03T15:04:05Z","epoch":0,"release":"4.fc30","source":{"license":"MIT","version":"4.1"}},{"arch":"x86_64","build_time":"2006-05-02T15:04:05Z","epoch":0,"release":"4.fc30","source":{"license":"MIT","version":"4.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package5","summary":"pkg5 sum","description":"pkg5 desc","homepage":"https://pkg5.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-06-02T15:04:05Z","epoch":0,"release":"5.fc30","source":{"license":"MIT","version":"5.0"}},{"arch":"x86_64","build_time":"2006-06-03T15:04:05Z","epoch":0,"release":"5.fc30","source":{"license":"MIT","version":"5.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package6","summary":"pkg6 sum","description":"pkg6 desc","homepage":"https://pkg6.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-07-02T15:04:05Z","epoch":0,"release":"6.fc30","source":{"license":"MIT","version":"6.0"}},{"arch":"x86_64","build_time":"2006-07-03T15:04:05Z","epoch":0,"release":"6.fc30","source":{"license":"MIT","version":"6.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package7","summary":"pkg7 sum","description":"pkg7 desc","homepage":"https://pkg7.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-08-03T15:04:05Z","epoch":0,"release":"7.fc30","source":{"license":"MIT","version":"7.1"}},{"arch":"x86_64","build_time":"2006-08-02T15:04:05Z","epoch":0,"release":"7.fc30","source":{"license":"MIT","version":"7.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package8","summary":"pkg8 sum","description":"pkg8 desc","homepage":"https://pkg8.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-09-03T15:04:05Z","epoch":0,"release":"8.fc30","source":{"license":"MIT","version":"8.1"}},{"arch":"x86_64","build_time":"2006-09-02T15:04:05Z","epoch":0,"release":"8.fc30","source":{"license":"MIT","version":"8.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package9","summary":"pkg9 sum","description":"pkg9 desc","homepage":"https://pkg9.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-10-03T15:04:05Z","epoch":0,"release":"9.fc30","source":{"license":"MIT","version":"9.1"}},{"arch":"x86_64","build_time":"2006-10-02T15:04:05Z","epoch":0,"release":"9.fc30","source":{"license":"MIT","version":"9.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]}]}`},
		{rpmmd_mock.BadFetch, "/api/v0/modules/info/package2*,package16", http.StatusBadRequest, `{"status":false,"errors":[{"id":"ModulesError","msg":"msg: DNF error occured: FetchError: There was a problem when fetching packages."}]}`},
	}

	for _, c := range cases {
		api, _ := createWeldrAPI(c.Fixture)
		testRoute(t, api, true, "GET", c.Path, ``, c.ExpectedStatus, c.ExpectedJSON)
	}
}

func TestProjectsList(t *testing.T) {
	var cases = []struct {
		Fixture        rpmmd_mock.Fixture
		Path           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{rpmmd_mock.BaseFixture, "/api/v0/projects/list", http.StatusOK, `{"total":22,"offset":0,"limit":20,"projects":[{"name":"package0","summary":"pkg0 sum","description":"pkg0 desc","homepage":"https://pkg0.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-01-03T15:04:05Z","epoch":0,"release":"0.fc30","source":{"license":"MIT","version":"0.1"}},{"arch":"x86_64","build_time":"2006-01-02T15:04:05Z","epoch":0,"release":"0.fc30","source":{"license":"MIT","version":"0.0"}}]},{"name":"package1","summary":"pkg1 sum","description":"pkg1 desc","homepage":"https://pkg1.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-02-02T15:04:05Z","epoch":0,"release":"1.fc30","source":{"license":"MIT","version":"1.0"}},{"arch":"x86_64","build_time":"2006-02-03T15:04:05Z","epoch":0,"release":"1.fc30","source":{"license":"MIT","version":"1.1"}}]},{"name":"package10","summary":"pkg10 sum","description":"pkg10 desc","homepage":"https://pkg10.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-11-02T15:04:05Z","epoch":0,"release":"10.fc30","source":{"license":"MIT","version":"10.0"}},{"arch":"x86_64","build_time":"2006-11-03T15:04:05Z","epoch":0,"release":"10.fc30","source":{"license":"MIT","version":"10.1"}}]},{"name":"package11","summary":"pkg11 sum","description":"pkg11 desc","homepage":"https://pkg11.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-12-03T15:04:05Z","epoch":0,"release":"11.fc30","source":{"license":"MIT","version":"11.1"}},{"arch":"x86_64","build_time":"2006-12-02T15:04:05Z","epoch":0,"release":"11.fc30","source":{"license":"MIT","version":"11.0"}}]},{"name":"package12","summary":"pkg12 sum","description":"pkg12 desc","homepage":"https://pkg12.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-01-02T15:04:05Z","epoch":0,"release":"12.fc30","source":{"license":"MIT","version":"12.0"}},{"arch":"x86_64","build_time":"2007-01-03T15:04:05Z","epoch":0,"release":"12.fc30","source":{"license":"MIT","version":"12.1"}}]},{"name":"package13","summary":"pkg13 sum","description":"pkg13 desc","homepage":"https://pkg13.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-02-02T15:04:05Z","epoch":0,"release":"13.fc30","source":{"license":"MIT","version":"13.0"}},{"arch":"x86_64","build_time":"2007-02-03T15:04:05Z","epoch":0,"release":"13.fc30","source":{"license":"MIT","version":"13.1"}}]},{"name":"package14","summary":"pkg14 sum","description":"pkg14 desc","homepage":"https://pkg14.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-03-03T15:04:05Z","epoch":0,"release":"14.fc30","source":{"license":"MIT","version":"14.1"}},{"arch":"x86_64","build_time":"2007-03-02T15:04:05Z","epoch":0,"release":"14.fc30","source":{"license":"MIT","version":"14.0"}}]},{"name":"package15","summary":"pkg15 sum","description":"pkg15 desc","homepage":"https://pkg15.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-04-03T15:04:05Z","epoch":0,"release":"15.fc30","source":{"license":"MIT","version":"15.1"}},{"arch":"x86_64","build_time":"2007-04-02T15:04:05Z","epoch":0,"release":"15.fc30","source":{"license":"MIT","version":"15.0"}}]},{"name":"package16","summary":"pkg16 sum","description":"pkg16 desc","homepage":"https://pkg16.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-05-02T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.0"}},{"arch":"x86_64","build_time":"2007-05-03T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.1"}}]},{"name":"package17","summary":"pkg17 sum","description":"pkg17 desc","homepage":"https://pkg17.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-06-03T15:04:05Z","epoch":0,"release":"17.fc30","source":{"license":"MIT","version":"17.1"}},{"arch":"x86_64","build_time":"2007-06-02T15:04:05Z","epoch":0,"release":"17.fc30","source":{"license":"MIT","version":"17.0"}}]},{"name":"package18","summary":"pkg18 sum","description":"pkg18 desc","homepage":"https://pkg18.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-07-02T15:04:05Z","epoch":0,"release":"18.fc30","source":{"license":"MIT","version":"18.0"}},{"arch":"x86_64","build_time":"2007-07-03T15:04:05Z","epoch":0,"release":"18.fc30","source":{"license":"MIT","version":"18.1"}}]},{"name":"package19","summary":"pkg19 sum","description":"pkg19 desc","homepage":"https://pkg19.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-08-03T15:04:05Z","epoch":0,"release":"19.fc30","source":{"license":"MIT","version":"19.1"}},{"arch":"x86_64","build_time":"2007-08-02T15:04:05Z","epoch":0,"release":"19.fc30","source":{"license":"MIT","version":"19.0"}}]},{"name":"package2","summary":"pkg2 sum","description":"pkg2 desc","homepage":"https://pkg2.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-03-02T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.0"}},{"arch":"x86_64","build_time":"2006-03-03T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.1"}}]},{"name":"package20","summary":"pkg20 sum","description":"pkg20 desc","homepage":"https://pkg20.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-09-03T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.1"}},{"arch":"x86_64","build_time":"2007-09-02T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.0"}}]},{"name":"package21","summary":"pkg21 sum","description":"pkg21 desc","homepage":"https://pkg21.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-10-02T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.0"}},{"arch":"x86_64","build_time":"2007-10-03T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.1"}}]},{"name":"package3","summary":"pkg3 sum","description":"pkg3 desc","homepage":"https://pkg3.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-04-03T15:04:05Z","epoch":0,"release":"3.fc30","source":{"license":"MIT","version":"3.1"}},{"arch":"x86_64","build_time":"2006-04-02T15:04:05Z","epoch":0,"release":"3.fc30","source":{"license":"MIT","version":"3.0"}}]},{"name":"package4","summary":"pkg4 sum","description":"pkg4 desc","homepage":"https://pkg4.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-05-03T15:04:05Z","epoch":0,"release":"4.fc30","source":{"license":"MIT","version":"4.1"}},{"arch":"x86_64","build_time":"2006-05-02T15:04:05Z","epoch":0,"release":"4.fc30","source":{"license":"MIT","version":"4.0"}}]},{"name":"package5","summary":"pkg5 sum","description":"pkg5 desc","homepage":"https://pkg5.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-06-03T15:04:05Z","epoch":0,"release":"5.fc30","source":{"license":"MIT","version":"5.1"}},{"arch":"x86_64","build_time":"2006-06-02T15:04:05Z","epoch":0,"release":"5.fc30","source":{"license":"MIT","version":"5.0"}}]},{"name":"package6","summary":"pkg6 sum","description":"pkg6 desc","homepage":"https://pkg6.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-07-02T15:04:05Z","epoch":0,"release":"6.fc30","source":{"license":"MIT","version":"6.0"}},{"arch":"x86_64","build_time":"2006-07-03T15:04:05Z","epoch":0,"release":"6.fc30","source":{"license":"MIT","version":"6.1"}}]},{"name":"package7","summary":"pkg7 sum","description":"pkg7 desc","homepage":"https://pkg7.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-08-02T15:04:05Z","epoch":0,"release":"7.fc30","source":{"license":"MIT","version":"7.0"}},{"arch":"x86_64","build_time":"2006-08-03T15:04:05Z","epoch":0,"release":"7.fc30","source":{"license":"MIT","version":"7.1"}}]}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/projects/list/", http.StatusOK, `{"total":22,"offset":0,"limit":20,"projects":[{"name":"package0","summary":"pkg0 sum","description":"pkg0 desc","homepage":"https://pkg0.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-01-03T15:04:05Z","epoch":0,"release":"0.fc30","source":{"license":"MIT","version":"0.1"}},{"arch":"x86_64","build_time":"2006-01-02T15:04:05Z","epoch":0,"release":"0.fc30","source":{"license":"MIT","version":"0.0"}}]},{"name":"package1","summary":"pkg1 sum","description":"pkg1 desc","homepage":"https://pkg1.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-02-02T15:04:05Z","epoch":0,"release":"1.fc30","source":{"license":"MIT","version":"1.0"}},{"arch":"x86_64","build_time":"2006-02-03T15:04:05Z","epoch":0,"release":"1.fc30","source":{"license":"MIT","version":"1.1"}}]},{"name":"package10","summary":"pkg10 sum","description":"pkg10 desc","homepage":"https://pkg10.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-11-02T15:04:05Z","epoch":0,"release":"10.fc30","source":{"license":"MIT","version":"10.0"}},{"arch":"x86_64","build_time":"2006-11-03T15:04:05Z","epoch":0,"release":"10.fc30","source":{"license":"MIT","version":"10.1"}}]},{"name":"package11","summary":"pkg11 sum","description":"pkg11 desc","homepage":"https://pkg11.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-12-03T15:04:05Z","epoch":0,"release":"11.fc30","source":{"license":"MIT","version":"11.1"}},{"arch":"x86_64","build_time":"2006-12-02T15:04:05Z","epoch":0,"release":"11.fc30","source":{"license":"MIT","version":"11.0"}}]},{"name":"package12","summary":"pkg12 sum","description":"pkg12 desc","homepage":"https://pkg12.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-01-02T15:04:05Z","epoch":0,"release":"12.fc30","source":{"license":"MIT","version":"12.0"}},{"arch":"x86_64","build_time":"2007-01-03T15:04:05Z","epoch":0,"release":"12.fc30","source":{"license":"MIT","version":"12.1"}}]},{"name":"package13","summary":"pkg13 sum","description":"pkg13 desc","homepage":"https://pkg13.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-02-02T15:04:05Z","epoch":0,"release":"13.fc30","source":{"license":"MIT","version":"13.0"}},{"arch":"x86_64","build_time":"2007-02-03T15:04:05Z","epoch":0,"release":"13.fc30","source":{"license":"MIT","version":"13.1"}}]},{"name":"package14","summary":"pkg14 sum","description":"pkg14 desc","homepage":"https://pkg14.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-03-03T15:04:05Z","epoch":0,"release":"14.fc30","source":{"license":"MIT","version":"14.1"}},{"arch":"x86_64","build_time":"2007-03-02T15:04:05Z","epoch":0,"release":"14.fc30","source":{"license":"MIT","version":"14.0"}}]},{"name":"package15","summary":"pkg15 sum","description":"pkg15 desc","homepage":"https://pkg15.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-04-03T15:04:05Z","epoch":0,"release":"15.fc30","source":{"license":"MIT","version":"15.1"}},{"arch":"x86_64","build_time":"2007-04-02T15:04:05Z","epoch":0,"release":"15.fc30","source":{"license":"MIT","version":"15.0"}}]},{"name":"package16","summary":"pkg16 sum","description":"pkg16 desc","homepage":"https://pkg16.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-05-02T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.0"}},{"arch":"x86_64","build_time":"2007-05-03T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.1"}}]},{"name":"package17","summary":"pkg17 sum","description":"pkg17 desc","homepage":"https://pkg17.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-06-03T15:04:05Z","epoch":0,"release":"17.fc30","source":{"license":"MIT","version":"17.1"}},{"arch":"x86_64","build_time":"2007-06-02T15:04:05Z","epoch":0,"release":"17.fc30","source":{"license":"MIT","version":"17.0"}}]},{"name":"package18","summary":"pkg18 sum","description":"pkg18 desc","homepage":"https://pkg18.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-07-02T15:04:05Z","epoch":0,"release":"18.fc30","source":{"license":"MIT","version":"18.0"}},{"arch":"x86_64","build_time":"2007-07-03T15:04:05Z","epoch":0,"release":"18.fc30","source":{"license":"MIT","version":"18.1"}}]},{"name":"package19","summary":"pkg19 sum","description":"pkg19 desc","homepage":"https://pkg19.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-08-03T15:04:05Z","epoch":0,"release":"19.fc30","source":{"license":"MIT","version":"19.1"}},{"arch":"x86_64","build_time":"2007-08-02T15:04:05Z","epoch":0,"release":"19.fc30","source":{"license":"MIT","version":"19.0"}}]},{"name":"package2","summary":"pkg2 sum","description":"pkg2 desc","homepage":"https://pkg2.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-03-02T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.0"}},{"arch":"x86_64","build_time":"2006-03-03T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.1"}}]},{"name":"package20","summary":"pkg20 sum","description":"pkg20 desc","homepage":"https://pkg20.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-09-03T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.1"}},{"arch":"x86_64","build_time":"2007-09-02T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.0"}}]},{"name":"package21","summary":"pkg21 sum","description":"pkg21 desc","homepage":"https://pkg21.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-10-02T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.0"}},{"arch":"x86_64","build_time":"2007-10-03T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.1"}}]},{"name":"package3","summary":"pkg3 sum","description":"pkg3 desc","homepage":"https://pkg3.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-04-03T15:04:05Z","epoch":0,"release":"3.fc30","source":{"license":"MIT","version":"3.1"}},{"arch":"x86_64","build_time":"2006-04-02T15:04:05Z","epoch":0,"release":"3.fc30","source":{"license":"MIT","version":"3.0"}}]},{"name":"package4","summary":"pkg4 sum","description":"pkg4 desc","homepage":"https://pkg4.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-05-03T15:04:05Z","epoch":0,"release":"4.fc30","source":{"license":"MIT","version":"4.1"}},{"arch":"x86_64","build_time":"2006-05-02T15:04:05Z","epoch":0,"release":"4.fc30","source":{"license":"MIT","version":"4.0"}}]},{"name":"package5","summary":"pkg5 sum","description":"pkg5 desc","homepage":"https://pkg5.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-06-03T15:04:05Z","epoch":0,"release":"5.fc30","source":{"license":"MIT","version":"5.1"}},{"arch":"x86_64","build_time":"2006-06-02T15:04:05Z","epoch":0,"release":"5.fc30","source":{"license":"MIT","version":"5.0"}}]},{"name":"package6","summary":"pkg6 sum","description":"pkg6 desc","homepage":"https://pkg6.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-07-02T15:04:05Z","epoch":0,"release":"6.fc30","source":{"license":"MIT","version":"6.0"}},{"arch":"x86_64","build_time":"2006-07-03T15:04:05Z","epoch":0,"release":"6.fc30","source":{"license":"MIT","version":"6.1"}}]},{"name":"package7","summary":"pkg7 sum","description":"pkg7 desc","homepage":"https://pkg7.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-08-02T15:04:05Z","epoch":0,"release":"7.fc30","source":{"license":"MIT","version":"7.0"}},{"arch":"x86_64","build_time":"2006-08-03T15:04:05Z","epoch":0,"release":"7.fc30","source":{"license":"MIT","version":"7.1"}}]}]}`},
		{rpmmd_mock.BadFetch, "/api/v0/projects/list/", http.StatusBadRequest, `{"status":false,"errors":[{"id":"ProjectsError","msg":"msg: DNF error occured: FetchError: There was a problem when fetching packages."}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/projects/list?offset=1&limit=1", http.StatusOK, `{"total":22,"offset":1,"limit":1,"projects":[{"name":"package1","summary":"pkg1 sum","description":"pkg1 desc","homepage":"https://pkg1.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-02-02T15:04:05Z","epoch":0,"release":"1.fc30","source":{"license":"MIT","version":"1.0"}},{"arch":"x86_64","build_time":"2006-02-03T15:04:05Z","epoch":0,"release":"1.fc30","source":{"license":"MIT","version":"1.1"}}]}]}`},
	}

	for _, c := range cases {
		api, _ := createWeldrAPI(c.Fixture)
		testRoute(t, api, true, "GET", c.Path, ``, c.ExpectedStatus, c.ExpectedJSON)
	}
}

func TestModulesList(t *testing.T) {
	var cases = []struct {
		Fixture        rpmmd_mock.Fixture
		Path           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{rpmmd_mock.BaseFixture, "/api/v0/modules/list", http.StatusOK, `{"total":22,"offset":0,"limit":20,"modules":[{"name":"package0","group_type":"rpm"},{"name":"package1","group_type":"rpm"},{"name":"package10","group_type":"rpm"},{"name":"package11","group_type":"rpm"},{"name":"package12","group_type":"rpm"},{"name":"package13","group_type":"rpm"},{"name":"package14","group_type":"rpm"},{"name":"package15","group_type":"rpm"},{"name":"package16","group_type":"rpm"},{"name":"package17","group_type":"rpm"},{"name":"package18","group_type":"rpm"},{"name":"package19","group_type":"rpm"},{"name":"package2","group_type":"rpm"},{"name":"package20","group_type":"rpm"},{"name":"package21","group_type":"rpm"},{"name":"package3","group_type":"rpm"},{"name":"package4","group_type":"rpm"},{"name":"package5","group_type":"rpm"},{"name":"package6","group_type":"rpm"},{"name":"package7","group_type":"rpm"}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/modules/list/", http.StatusOK, `{"total":22,"offset":0,"limit":20,"modules":[{"name":"package0","group_type":"rpm"},{"name":"package1","group_type":"rpm"},{"name":"package10","group_type":"rpm"},{"name":"package11","group_type":"rpm"},{"name":"package12","group_type":"rpm"},{"name":"package13","group_type":"rpm"},{"name":"package14","group_type":"rpm"},{"name":"package15","group_type":"rpm"},{"name":"package16","group_type":"rpm"},{"name":"package17","group_type":"rpm"},{"name":"package18","group_type":"rpm"},{"name":"package19","group_type":"rpm"},{"name":"package2","group_type":"rpm"},{"name":"package20","group_type":"rpm"},{"name":"package21","group_type":"rpm"},{"name":"package3","group_type":"rpm"},{"name":"package4","group_type":"rpm"},{"name":"package5","group_type":"rpm"},{"name":"package6","group_type":"rpm"},{"name":"package7","group_type":"rpm"}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/modules/list/nonexistingpkg", http.StatusBadRequest, `{"status":false,"errors":[{"id":"UnknownModule","msg":"No packages have been found."}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/modules/list/package2*,package16", http.StatusOK, `{"total":4,"offset":0,"limit":20,"modules":[{"name":"package16","group_type":"rpm"},{"name":"package2","group_type":"rpm"},{"name":"package20","group_type":"rpm"},{"name":"package21","group_type":"rpm"}]}`},
		{rpmmd_mock.BadFetch, "/api/v0/modules/list/package2*,package16", http.StatusBadRequest, `{"status":false,"errors":[{"id":"ModulesError","msg":"msg: DNF error occured: FetchError: There was a problem when fetching packages."}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/modules/list/package2*,package16?offset=1&limit=1", http.StatusOK, `{"total":4,"offset":1,"limit":1,"modules":[{"name":"package2","group_type":"rpm"}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/modules/list/*", http.StatusOK, `{"total":22,"offset":0,"limit":20,"modules":[{"name":"package0","group_type":"rpm"},{"name":"package1","group_type":"rpm"},{"name":"package10","group_type":"rpm"},{"name":"package11","group_type":"rpm"},{"name":"package12","group_type":"rpm"},{"name":"package13","group_type":"rpm"},{"name":"package14","group_type":"rpm"},{"name":"package15","group_type":"rpm"},{"name":"package16","group_type":"rpm"},{"name":"package17","group_type":"rpm"},{"name":"package18","group_type":"rpm"},{"name":"package19","group_type":"rpm"},{"name":"package2","group_type":"rpm"},{"name":"package20","group_type":"rpm"},{"name":"package21","group_type":"rpm"},{"name":"package3","group_type":"rpm"},{"name":"package4","group_type":"rpm"},{"name":"package5","group_type":"rpm"},{"name":"package6","group_type":"rpm"},{"name":"package7","group_type":"rpm"}]}`},
	}

	for _, c := range cases {
		api, _ := createWeldrAPI(c.Fixture)
		testRoute(t, api, true, "GET", c.Path, ``, c.ExpectedStatus, c.ExpectedJSON)
	}
}
