package weldr_test

import (
	"archive/tar"
	"bytes"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	_ "github.com/osbuild/osbuild-composer/internal/distro/test"
	rpmmd_mock "github.com/osbuild/osbuild-composer/internal/mocks/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/test"
	"github.com/osbuild/osbuild-composer/internal/weldr"

	"github.com/BurntSushi/toml"
	"github.com/google/go-cmp/cmp"
)

func createWeldrAPI(fixtureGenerator rpmmd_mock.FixtureGenerator) (*weldr.API, *store.Store) {
	fixture := fixtureGenerator()
	rpm := rpmmd_mock.NewRPMMDMock(fixture)
	d := distro.New("test")

	return weldr.New(rpm, d, nil, fixture.Store), fixture.Store
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

		{"/api/v0/blueprints/list", http.StatusOK, `{"total":1,"offset":0,"limit":1,"blueprints":["test"]}`},
		{"/api/v0/blueprints/info/", http.StatusNotFound, `{"errors":[{"code":404,"id":"HTTPError","msg":"Not Found"}],"status":false}`},
		{"/api/v0/blueprints/info/foo", http.StatusBadRequest, `{"errors":[{"id":"UnknownBlueprint","msg":"foo: "}],"status":false}`},
	}

	for _, c := range cases {
		api, _ := createWeldrAPI(rpmmd_mock.BaseFixture)
		test.TestRoute(t, api, true, "GET", c.Path, ``, c.ExpectedStatus, c.ExpectedJSON)
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
		test.TestRoute(t, api, true, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
	}
}

func TestBlueprintsNewToml(t *testing.T) {
	blueprint := `
name = "test"
description = "Test"
version = "0.0.0"

[[packages]]
name = "httpd"
version = "2.4.*"`

	req := httptest.NewRequest("POST", "/api/v0/blueprints/new", bytes.NewReader([]byte(blueprint)))
	req.Header.Set("Content-Type", "text/x-toml")
	recorder := httptest.NewRecorder()

	api, _ := createWeldrAPI(rpmmd_mock.BaseFixture)
	api.ServeHTTP(recorder, req)

	r := recorder.Result()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %v", r.StatusCode)
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
		test.SendHTTP(api, true, "POST", "/api/v0/blueprints/new", `{"name":"test","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
		test.TestRoute(t, api, true, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
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
		test.SendHTTP(api, true, "POST", "/api/v0/blueprints/new", `{"name":"test1","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
		test.SendHTTP(api, true, "POST", "/api/v0/blueprints/new", `{"name":"test2","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
		test.SendHTTP(api, true, "POST", "/api/v0/blueprints/workspace", `{"name":"test2","description":"Test","packages":[{"name":"systemd","version":"123"}],"version":"0.0.0"}`)
		test.TestRoute(t, api, true, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
		test.SendHTTP(api, true, "DELETE", "/api/v0/blueprints/delete/test2", ``)
		test.SendHTTP(api, true, "DELETE", "/api/v0/blueprints/delete/test1", ``)
	}
}

func TestBlueprintsInfoToml(t *testing.T) {
	api, _ := createWeldrAPI(rpmmd_mock.BaseFixture)
	test.SendHTTP(api, true, "POST", "/api/v0/blueprints/new", `{"name":"test1","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)

	req := httptest.NewRequest("GET", "/api/v0/blueprints/info/test1?format=toml", nil)
	recorder := httptest.NewRecorder()
	api.ServeHTTP(recorder, req)

	resp := recorder.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %v", resp.StatusCode)
	}

	var got blueprint.Blueprint
	_, err := toml.DecodeReader(resp.Body, &got)
	if err != nil {
		t.Fatalf("error decoding toml file: %v", err)
	}

	expected := blueprint.Blueprint{
		Name:        "test1",
		Description: "Test",
		Version:     "0.0.0",
		Packages: []blueprint.Package{
			{"httpd", "2.4.*"},
		},
	}
	if diff := cmp.Diff(got, expected); diff != "" {
		t.Fatalf("received unexpected blueprint: %s", diff)
	}
}

func TestBlueprintsFreeze(t *testing.T) {
	var cases = []struct {
		Fixture        rpmmd_mock.FixtureGenerator
		Path           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{rpmmd_mock.BaseFixture, "/api/v0/blueprints/freeze/test", http.StatusOK, `{"blueprints":[{"blueprint":{"name":"test","description":"Test","version":"0.0.1","packages":[{"name":"dep-package1","version":"1.33-2.fc30.x86_64"}],"modules":[],"groups":[]}}],"errors":[]}`},
	}

	for _, c := range cases {
		api, _ := createWeldrAPI(c.Fixture)
		test.SendHTTP(api, false, "POST", "/api/v0/blueprints/new", `{"name":"test","description":"Test","packages":[{"name":"dep-package1","version":"*"}],"version":"0.0.0"}`)
		test.TestRoute(t, api, false, "GET", c.Path, ``, c.ExpectedStatus, c.ExpectedJSON)
		test.SendHTTP(api, false, "DELETE", "/api/v0/blueprints/delete/test", ``)
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
		test.SendHTTP(api, true, "POST", "/api/v0/blueprints/new", `{"name":"test","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
		test.SendHTTP(api, true, "POST", "/api/v0/blueprints/workspace", `{"name":"test","description":"Test","packages":[{"name":"systemd","version":"123"}],"version":"0.0.0"}`)
		test.TestRoute(t, api, true, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
		test.SendHTTP(api, true, "DELETE", "/api/v0/blueprints/delete/test", ``)
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
		test.SendHTTP(api, true, "POST", "/api/v0/blueprints/new", `{"name":"test","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
		test.TestRoute(t, api, true, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
		test.SendHTTP(api, true, "DELETE", "/api/v0/blueprints/delete/test", ``)
	}
}

func TestBlueprintsChanges(t *testing.T) {
	api, _ := createWeldrAPI(rpmmd_mock.BaseFixture)
	rand.Seed(time.Now().UnixNano())
	id := strconv.Itoa(rand.Int())
	ignoreFields := []string{"commit", "timestamp"}

	test.SendHTTP(api, true, "POST", "/api/v0/blueprints/new", `{"name":"`+id+`","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
	test.TestRoute(t, api, true, "GET", "/api/v0/blueprints/changes/failing"+id, ``, http.StatusOK, `{"blueprints":[],"errors":[{"id":"UnknownBlueprint","msg":"failing`+id+`"}],"limit":20,"offset":0}`, ignoreFields...)
	test.TestRoute(t, api, true, "GET", "/api/v0/blueprints/changes/"+id, ``, http.StatusOK, `{"blueprints":[{"changes":[{"commit":"","message":"Recipe `+id+`, version 0.0.0 saved.","revision":null,"timestamp":""}],"name":"`+id+`","total":1}],"errors":[],"limit":20,"offset":0}`, ignoreFields...)
	test.SendHTTP(api, true, "DELETE", "/api/v0/blueprints/delete/"+id, ``)
	test.SendHTTP(api, true, "POST", "/api/v0/blueprints/new", `{"name":"`+id+`","description":"Test","packages":[{"name":"httpd","version":"2.4.*"}],"version":"0.0.0"}`)
	test.TestRoute(t, api, true, "GET", "/api/v0/blueprints/changes/"+id, ``, http.StatusOK, `{"blueprints":[{"changes":[{"commit":"","message":"Recipe `+id+`, version 0.0.0 saved.","revision":null,"timestamp":""},{"commit":"","message":"Recipe `+id+`, version 0.0.0 saved.","revision":null,"timestamp":""}],"name":"`+id+`","total":2}],"errors":[],"limit":20,"offset":0}`, ignoreFields...)
	test.SendHTTP(api, true, "DELETE", "/api/v0/blueprints/delete/"+id, ``)
}

func TestCompose(t *testing.T) {
	expectedComposeLocal := &store.Compose{
		QueueStatus: "WAITING",
		Blueprint: &blueprint.Blueprint{
			Name:           "test",
			Version:        "0.0.0",
			Packages:       []blueprint.Package{},
			Modules:        []blueprint.Package{},
			Groups:         []blueprint.Group{},
			Customizations: nil,
		},
		OutputType: "tar",
		Targets:    []*target.Target{},
	}

	expectedComposeLocalAndAws := &store.Compose{
		QueueStatus: "WAITING",
		Blueprint: &blueprint.Blueprint{
			Name:           "test",
			Version:        "0.0.0",
			Packages:       []blueprint.Package{},
			Modules:        []blueprint.Package{},
			Groups:         []blueprint.Group{},
			Customizations: nil,
		},
		OutputType: "tar",
		Targets: []*target.Target{
			{
				Name:      "org.osbuild.aws",
				Status:    "WAITING",
				ImageName: "test_upload",
				Options: &target.AWSTargetOptions{
					Region:          "frankfurt",
					AccessKeyID:     "accesskey",
					SecretAccessKey: "secretkey",
					Bucket:          "clay",
					Key:             "imagekey",
				},
			},
		},
	}

	var cases = []struct {
		External        bool
		Method          string
		Path            string
		Body            string
		ExpectedStatus  int
		ExpectedJSON    string
		ExpectedCompose *store.Compose
		IgnoreFields    []string
	}{
		{true, "POST", "/api/v0/compose", `{"blueprint_name": "http-server","compose_type": "tar","branch": "master"}`, http.StatusBadRequest, `{"status":false,"errors":[{"id":"UnknownBlueprint","msg":"Unknown blueprint name: http-server"}]}`, nil, []string{"build_id"}},
		{false, "POST", "/api/v0/compose", `{"blueprint_name": "test","compose_type": "tar","branch": "master"}`, http.StatusOK, `{"status": true}`, expectedComposeLocal, []string{"build_id"}},
		{false, "POST", "/api/v1/compose", `{"blueprint_name": "test","compose_type":"tar","branch":"master","upload":{"image_name":"test_upload","provider":"aws","settings":{"region":"frankfurt","accessKeyID":"accesskey","secretAccessKey":"secretkey","bucket":"clay","key":"imagekey"}}}`, http.StatusOK, `{"status": true}`, expectedComposeLocalAndAws, []string{"build_id"}},
	}

	for _, c := range cases {
		api, s := createWeldrAPI(rpmmd_mock.NoComposesFixture)
		test.TestRoute(t, api, c.External, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON, c.IgnoreFields...)

		if c.ExpectedStatus != http.StatusOK {
			continue
		}

		if len(s.Composes) != 1 {
			t.Fatalf("%s: bad compose count in store: %d", c.Path, len(s.Composes))
		}

		// I have no idea how to get the compose in better way
		var compose store.Compose
		for _, c := range s.Composes {
			compose = c
			break
		}

		if diff := cmp.Diff(compose, *c.ExpectedCompose, test.IgnoreDates(), test.IgnoreUuids(), test.Ignore("Targets.Options.Location")); diff != "" {
			t.Errorf("%s: compose in store isn't the same as expected, diff:\n%s", c.Path, diff)
		}

	}
}

func TestComposeStatus(t *testing.T) {
	var cases = []struct {
		Fixture        rpmmd_mock.FixtureGenerator
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{rpmmd_mock.BaseFixture, "GET", "/api/v0/compose/status/30000000-0000-0000-0000-000000000000,30000000-0000-0000-0000-000000000002", ``, http.StatusOK, `{"uuids":[{"id":"30000000-0000-0000-0000-000000000000","blueprint":"test","version":"0.0.0","compose_type":"tar","image_size":0,"queue_status":"WAITING","job_created":1574857140},{"id":"30000000-0000-0000-0000-000000000002","blueprint":"test","version":"0.0.0","compose_type":"tar","image_size":0,"queue_status":"FINISHED","job_created":1574857140,"job_started":1574857140,"job_finished":1574857140}]}`},
		{rpmmd_mock.BaseFixture, "GET", "/api/v0/compose/status/*", ``, http.StatusOK, `{"uuids":[{"id":"30000000-0000-0000-0000-000000000000","blueprint":"test","version":"0.0.0","compose_type":"tar","image_size":0,"queue_status":"WAITING","job_created":1574857140},{"id":"30000000-0000-0000-0000-000000000001","blueprint":"test","version":"0.0.0","compose_type":"tar","image_size":0,"queue_status":"RUNNING","job_created":1574857140,"job_started":1574857140},{"id":"30000000-0000-0000-0000-000000000002","blueprint":"test","version":"0.0.0","compose_type":"tar","image_size":0,"queue_status":"FINISHED","job_created":1574857140,"job_started":1574857140,"job_finished":1574857140},{"id":"30000000-0000-0000-0000-000000000003","blueprint":"test","version":"0.0.0","compose_type":"tar","image_size":0,"queue_status":"FAILED","job_created":1574857140,"job_started":1574857140,"job_finished":1574857140}]}`},
		{rpmmd_mock.BaseFixture, "GET", "/api/v1/compose/status/30000000-0000-0000-0000-000000000000", ``, http.StatusOK, `{"uuids":[{"id":"30000000-0000-0000-0000-000000000000","blueprint":"test","version":"0.0.0","compose_type":"tar","image_size":0,"queue_status":"WAITING","job_created":1574857140,"uploads":[{"uuid":"10000000-0000-0000-0000-000000000000","status":"WAITING","provider_name":"aws","image_name":"awsimage","creation_time":1574857140,"settings":{"region":"frankfurt","accessKeyID":"accesskey","secretAccessKey":"secretkey","bucket":"clay","key":"imagekey"}}]}]}`},
	}

	if len(os.Getenv("OSBUILD_COMPOSER_TEST_EXTERNAL")) > 0 {
		t.Skip("This test is for internal testing only")
	}

	for _, c := range cases {
		api, _ := createWeldrAPI(rpmmd_mock.BaseFixture)
		test.TestRoute(t, api, false, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON, "id", "job_created", "job_started")
	}
}

func TestComposeInfo(t *testing.T) {
	var cases = []struct {
		Fixture        rpmmd_mock.FixtureGenerator
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{rpmmd_mock.BaseFixture, "GET", "/api/v0/compose/info/30000000-0000-0000-0000-000000000000", ``, http.StatusOK, `{"id":"30000000-0000-0000-0000-000000000000","config":"","blueprint":{"name":"test","description":"","version":"0.0.0","packages":[],"modules":[],"groups":[]},"commit":"","deps":[],"compose_type":"tar","queue_status":"WAITING","image_size":0}`},
		{rpmmd_mock.BaseFixture, "GET", "/api/v1/compose/info/30000000-0000-0000-0000-000000000000", ``, http.StatusOK, `{"id":"30000000-0000-0000-0000-000000000000","config":"","blueprint":{"name":"test","description":"","version":"0.0.0","packages":[],"modules":[],"groups":[]},"commit":"","deps":[],"compose_type":"tar","queue_status":"WAITING","image_size":0,"uploads":[{"uuid":"10000000-0000-0000-0000-000000000000","status":"WAITING","provider_name":"aws","image_name":"awsimage","creation_time":1574857140,"settings":{"region":"frankfurt","accessKeyID":"accesskey","secretAccessKey":"secretkey","bucket":"clay","key":"imagekey"}}]}`},
		{rpmmd_mock.BaseFixture, "GET", "/api/v1/compose/info/30000000-0000-0000-0000", ``, http.StatusBadRequest, `{"status":false,"errors":[{"id":"UnknownUUID","msg":"30000000-0000-0000-0000 is not a valid build uuid"}]}`},
		{rpmmd_mock.BaseFixture, "GET", "/api/v1/compose/info/42000000-0000-0000-0000-000000000000", ``, http.StatusBadRequest, `{"status":false,"errors":[{"id":"UnknownUUID","msg":"42000000-0000-0000-0000-000000000000 is not a valid build uuid"}]}`},
	}

	if len(os.Getenv("OSBUILD_COMPOSER_TEST_EXTERNAL")) > 0 {
		t.Skip("This test is for internal testing only")
	}

	for _, c := range cases {
		api, _ := createWeldrAPI(rpmmd_mock.BaseFixture)
		test.TestRoute(t, api, false, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
	}
}

func TestComposeLogs(t *testing.T) {
	if len(os.Getenv("OSBUILD_COMPOSER_TEST_EXTERNAL")) > 0 {
		t.Skip("This test is for internal testing only")
	}

	var successCases = []struct {
		Path                       string
		ExpectedContentDisposition string
		ExpectedContentType        string
		ExpectedFileName           string
		ExpectedFileContent        string
	}{
		{"/api/v0/compose/logs/30000000-0000-0000-0000-000000000002", "attachment; filename=30000000-0000-0000-0000-000000000002-logs.tar", "application/x-tar", "logs/osbuild.log", "SUCCESS\n"},
		{"/api/v1/compose/logs/30000000-0000-0000-0000-000000000002", "attachment; filename=30000000-0000-0000-0000-000000000002-logs.tar", "application/x-tar", "logs/osbuild.log", "SUCCESS\n"},
	}

	for _, c := range successCases {
		api, _ := createWeldrAPI(rpmmd_mock.BaseFixture)

		response := test.SendHTTP(api, false, "GET", c.Path, "")
		if response.Header.Get("content-disposition") != c.ExpectedContentDisposition {
			t.Errorf("%s: expected content-disposition: %s, but got: %s", c.Path, c.ExpectedContentDisposition, response.Header.Get("content-disposition"))
		}

		if response.Header.Get("content-type") != c.ExpectedContentType {
			t.Errorf("%s: expected content-type: %s, but got: %s", c.Path, c.ExpectedContentType, response.Header.Get("content-type"))
		}

		tr := tar.NewReader(response.Body)
		h, err := tr.Next()

		if err != nil {
			t.Errorf("untarring failed with error: %s", err.Error())
		}

		if h.Name != c.ExpectedFileName {
			t.Errorf("%s: expected log content: %s, but got: %s", c.Path, c.ExpectedFileName, h.Name)
		}

		var buffer bytes.Buffer

		io.Copy(&buffer, tr)

		if buffer.String() != c.ExpectedFileContent {
			t.Errorf("%s: expected log content: %s, but got: %s", c.Path, c.ExpectedFileContent, buffer.String())
		}
	}

	var failureCases = []struct {
		Path         string
		ExpectedJSON string
	}{
		{"/api/v1/compose/logs/30000000-0000-0000-0000", `{"status":false,"errors":[{"id":"UnknownUUID","msg":"30000000-0000-0000-0000 is not a valid build uuid"}]}`},
		{"/api/v1/compose/logs/42000000-0000-0000-0000-000000000000", `{"status":false,"errors":[{"id":"UnknownUUID","msg":"Compose 42000000-0000-0000-0000-000000000000 doesn't exist"}]}`},
		{"/api/v1/compose/logs/30000000-0000-0000-0000-000000000000", `{"status":false,"errors":[{"id":"BuildInWrongState","msg":"Build 30000000-0000-0000-0000-000000000000 not in FINISHED or FAILED state."}]}`},
	}

	if len(os.Getenv("OSBUILD_COMPOSER_TEST_EXTERNAL")) > 0 {
		t.Skip("This test is for internal testing only")
	}

	for _, c := range failureCases {
		api, _ := createWeldrAPI(rpmmd_mock.BaseFixture)
		test.TestRoute(t, api, false, "GET", c.Path, "", http.StatusBadRequest, c.ExpectedJSON)
	}
}

func TestComposeQueue(t *testing.T) {
	var cases = []struct {
		Fixture        rpmmd_mock.FixtureGenerator
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{rpmmd_mock.BaseFixture, "GET", "/api/v0/compose/queue", ``, http.StatusOK, `{"new":[{"blueprint":"test","version":"0.0.0","compose_type":"tar","image_size":0,"queue_status":"WAITING"}],"run":[{"blueprint":"test","version":"0.0.0","compose_type":"tar","image_size":0,"queue_status":"RUNNING"}]}`},
		{rpmmd_mock.BaseFixture, "GET", "/api/v1/compose/queue", ``, http.StatusOK, `{"new":[{"blueprint":"test","version":"0.0.0","compose_type":"tar","image_size":0,"queue_status":"WAITING","uploads":[{"uuid":"10000000-0000-0000-0000-000000000000","status":"WAITING","provider_name":"aws","image_name":"awsimage","creation_time":1574857140,"settings":{"region":"frankfurt","accessKeyID":"accesskey","secretAccessKey":"secretkey","bucket":"clay","key":"imagekey"}}]}],"run":[{"blueprint":"test","version":"0.0.0","compose_type":"tar","image_size":0,"queue_status":"RUNNING"}]}`},
		{rpmmd_mock.NoComposesFixture, "GET", "/api/v0/compose/queue", ``, http.StatusOK, `{"new":[],"run":[]}`},
	}

	if len(os.Getenv("OSBUILD_COMPOSER_TEST_EXTERNAL")) > 0 {
		t.Skip("This test is for internal testing only")
	}

	for _, c := range cases {
		api, _ := createWeldrAPI(c.Fixture)
		test.TestRoute(t, api, false, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON, "id", "job_created", "job_started")
	}
}

func TestComposeFinished(t *testing.T) {
	var cases = []struct {
		Fixture        rpmmd_mock.FixtureGenerator
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{rpmmd_mock.BaseFixture, "GET", "/api/v0/compose/finished", ``, http.StatusOK, `{"finished":[{"id":"30000000-0000-0000-0000-000000000002","blueprint":"test","version":"0.0.0","compose_type":"tar","image_size":0,"queue_status":"FINISHED","job_created":1574857140,"job_started":1574857140,"job_finished":1574857140}]}`},
		{rpmmd_mock.BaseFixture, "GET", "/api/v1/compose/finished", ``, http.StatusOK, `{"finished":[{"id":"30000000-0000-0000-0000-000000000002","blueprint":"test","version":"0.0.0","compose_type":"tar","image_size":0,"queue_status":"FINISHED","job_created":1574857140,"job_started":1574857140,"job_finished":1574857140,"uploads":[{"uuid":"10000000-0000-0000-0000-000000000000","status":"WAITING","provider_name":"aws","image_name":"awsimage","creation_time":1574857140,"settings":{"region":"frankfurt","accessKeyID":"accesskey","secretAccessKey":"secretkey","bucket":"clay","key":"imagekey"}}]}]}`},
		{rpmmd_mock.NoComposesFixture, "GET", "/api/v0/compose/finished", ``, http.StatusOK, `{"finished":[]}`},
	}

	if len(os.Getenv("OSBUILD_COMPOSER_TEST_EXTERNAL")) > 0 {
		t.Skip("This test is for internal testing only")
	}

	for _, c := range cases {
		api, _ := createWeldrAPI(c.Fixture)
		test.TestRoute(t, api, false, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON, "id", "job_created", "job_started")
	}
}

func TestComposeFailed(t *testing.T) {
	var cases = []struct {
		Fixture        rpmmd_mock.FixtureGenerator
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{rpmmd_mock.BaseFixture, "GET", "/api/v0/compose/failed", ``, http.StatusOK, `{"failed":[{"id":"30000000-0000-0000-0000-000000000003","blueprint":"test","version":"0.0.0","compose_type":"tar","image_size":0,"queue_status":"FAILED","job_created":1574857140,"job_started":1574857140,"job_finished":1574857140}]}`},
		{rpmmd_mock.BaseFixture, "GET", "/api/v1/compose/failed", ``, http.StatusOK, `{"failed":[{"id":"30000000-0000-0000-0000-000000000003","blueprint":"test","version":"0.0.0","compose_type":"tar","image_size":0,"queue_status":"FAILED","job_created":1574857140,"job_started":1574857140,"job_finished":1574857140,"uploads":[{"uuid":"10000000-0000-0000-0000-000000000000","status":"WAITING","provider_name":"aws","image_name":"awsimage","creation_time":1574857140,"settings":{"region":"frankfurt","accessKeyID":"accesskey","secretAccessKey":"secretkey","bucket":"clay","key":"imagekey"}}]}]}`},
		{rpmmd_mock.NoComposesFixture, "GET", "/api/v0/compose/failed", ``, http.StatusOK, `{"failed":[]}`},
	}

	if len(os.Getenv("OSBUILD_COMPOSER_TEST_EXTERNAL")) > 0 {
		t.Skip("This test is for internal testing only")
	}

	for _, c := range cases {
		api, _ := createWeldrAPI(c.Fixture)
		test.TestRoute(t, api, false, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON, "id", "job_created", "job_started")
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
		test.TestRoute(t, api, true, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
		test.SendHTTP(api, true, "DELETE", "/api/v0/projects/source/delete/fish", ``)
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
		test.SendHTTP(api, true, "POST", "/api/v0/projects/source/new", `{"name": "fish","url": "https://download.opensuse.org/repositories/shells:/fish:/release:/3/Fedora_29/","type": "yum-baseurl","check_ssl": false,"check_gpg": false}`)
		test.TestRoute(t, api, true, c.Method, c.Path, c.Body, c.ExpectedStatus, c.ExpectedJSON)
		test.SendHTTP(api, true, "DELETE", "/api/v0/projects/source/delete/fish", ``)
	}
}

func TestProjectsDepsolve(t *testing.T) {
	var cases = []struct {
		Fixture        rpmmd_mock.FixtureGenerator
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
		test.TestRoute(t, api, true, "GET", c.Path, ``, c.ExpectedStatus, c.ExpectedJSON)
	}
}

func TestProjectsInfo(t *testing.T) {
	var cases = []struct {
		Fixture        rpmmd_mock.FixtureGenerator
		Path           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{rpmmd_mock.BaseFixture, "/api/v0/projects/info", http.StatusBadRequest, `{"status":false,"errors":[{"id":"UnknownProject","msg":"No packages specified."}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/projects/info/", http.StatusBadRequest, `{"status":false,"errors":[{"id":"UnknownProject","msg":"No packages specified."}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/projects/info/nonexistingpkg", http.StatusBadRequest, `{"status":false,"errors":[{"id":"UnknownProject","msg":"No packages have been found."}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/projects/info/*", http.StatusOK, `{"projects":[{"name":"package0","summary":"pkg0 sum","description":"pkg0 desc","homepage":"https://pkg0.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-01-03T15:04:05Z","epoch":0,"release":"0.fc30","source":{"license":"MIT","version":"0.1"}},{"arch":"x86_64","build_time":"2006-01-02T15:04:05Z","epoch":0,"release":"0.fc30","source":{"license":"MIT","version":"0.0"}}]},{"name":"package1","summary":"pkg1 sum","description":"pkg1 desc","homepage":"https://pkg1.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-02-02T15:04:05Z","epoch":0,"release":"1.fc30","source":{"license":"MIT","version":"1.0"}},{"arch":"x86_64","build_time":"2006-02-03T15:04:05Z","epoch":0,"release":"1.fc30","source":{"license":"MIT","version":"1.1"}}]},{"name":"package10","summary":"pkg10 sum","description":"pkg10 desc","homepage":"https://pkg10.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-11-02T15:04:05Z","epoch":0,"release":"10.fc30","source":{"license":"MIT","version":"10.0"}},{"arch":"x86_64","build_time":"2006-11-03T15:04:05Z","epoch":0,"release":"10.fc30","source":{"license":"MIT","version":"10.1"}}]},{"name":"package11","summary":"pkg11 sum","description":"pkg11 desc","homepage":"https://pkg11.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-12-03T15:04:05Z","epoch":0,"release":"11.fc30","source":{"license":"MIT","version":"11.1"}},{"arch":"x86_64","build_time":"2006-12-02T15:04:05Z","epoch":0,"release":"11.fc30","source":{"license":"MIT","version":"11.0"}}]},{"name":"package12","summary":"pkg12 sum","description":"pkg12 desc","homepage":"https://pkg12.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-01-02T15:04:05Z","epoch":0,"release":"12.fc30","source":{"license":"MIT","version":"12.0"}},{"arch":"x86_64","build_time":"2007-01-03T15:04:05Z","epoch":0,"release":"12.fc30","source":{"license":"MIT","version":"12.1"}}]},{"name":"package13","summary":"pkg13 sum","description":"pkg13 desc","homepage":"https://pkg13.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-02-02T15:04:05Z","epoch":0,"release":"13.fc30","source":{"license":"MIT","version":"13.0"}},{"arch":"x86_64","build_time":"2007-02-03T15:04:05Z","epoch":0,"release":"13.fc30","source":{"license":"MIT","version":"13.1"}}]},{"name":"package14","summary":"pkg14 sum","description":"pkg14 desc","homepage":"https://pkg14.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-03-03T15:04:05Z","epoch":0,"release":"14.fc30","source":{"license":"MIT","version":"14.1"}},{"arch":"x86_64","build_time":"2007-03-02T15:04:05Z","epoch":0,"release":"14.fc30","source":{"license":"MIT","version":"14.0"}}]},{"name":"package15","summary":"pkg15 sum","description":"pkg15 desc","homepage":"https://pkg15.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-04-03T15:04:05Z","epoch":0,"release":"15.fc30","source":{"license":"MIT","version":"15.1"}},{"arch":"x86_64","build_time":"2007-04-02T15:04:05Z","epoch":0,"release":"15.fc30","source":{"license":"MIT","version":"15.0"}}]},{"name":"package16","summary":"pkg16 sum","description":"pkg16 desc","homepage":"https://pkg16.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-05-02T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.0"}},{"arch":"x86_64","build_time":"2007-05-03T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.1"}}]},{"name":"package17","summary":"pkg17 sum","description":"pkg17 desc","homepage":"https://pkg17.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-06-03T15:04:05Z","epoch":0,"release":"17.fc30","source":{"license":"MIT","version":"17.1"}},{"arch":"x86_64","build_time":"2007-06-02T15:04:05Z","epoch":0,"release":"17.fc30","source":{"license":"MIT","version":"17.0"}}]},{"name":"package18","summary":"pkg18 sum","description":"pkg18 desc","homepage":"https://pkg18.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-07-02T15:04:05Z","epoch":0,"release":"18.fc30","source":{"license":"MIT","version":"18.0"}},{"arch":"x86_64","build_time":"2007-07-03T15:04:05Z","epoch":0,"release":"18.fc30","source":{"license":"MIT","version":"18.1"}}]},{"name":"package19","summary":"pkg19 sum","description":"pkg19 desc","homepage":"https://pkg19.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-08-03T15:04:05Z","epoch":0,"release":"19.fc30","source":{"license":"MIT","version":"19.1"}},{"arch":"x86_64","build_time":"2007-08-02T15:04:05Z","epoch":0,"release":"19.fc30","source":{"license":"MIT","version":"19.0"}}]},{"name":"package2","summary":"pkg2 sum","description":"pkg2 desc","homepage":"https://pkg2.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-03-02T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.0"}},{"arch":"x86_64","build_time":"2006-03-03T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.1"}}]},{"name":"package20","summary":"pkg20 sum","description":"pkg20 desc","homepage":"https://pkg20.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-09-03T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.1"}},{"arch":"x86_64","build_time":"2007-09-02T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.0"}}]},{"name":"package21","summary":"pkg21 sum","description":"pkg21 desc","homepage":"https://pkg21.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-10-02T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.0"}},{"arch":"x86_64","build_time":"2007-10-03T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.1"}}]},{"name":"package3","summary":"pkg3 sum","description":"pkg3 desc","homepage":"https://pkg3.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-04-03T15:04:05Z","epoch":0,"release":"3.fc30","source":{"license":"MIT","version":"3.1"}},{"arch":"x86_64","build_time":"2006-04-02T15:04:05Z","epoch":0,"release":"3.fc30","source":{"license":"MIT","version":"3.0"}}]},{"name":"package4","summary":"pkg4 sum","description":"pkg4 desc","homepage":"https://pkg4.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-05-03T15:04:05Z","epoch":0,"release":"4.fc30","source":{"license":"MIT","version":"4.1"}},{"arch":"x86_64","build_time":"2006-05-02T15:04:05Z","epoch":0,"release":"4.fc30","source":{"license":"MIT","version":"4.0"}}]},{"name":"package5","summary":"pkg5 sum","description":"pkg5 desc","homepage":"https://pkg5.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-06-03T15:04:05Z","epoch":0,"release":"5.fc30","source":{"license":"MIT","version":"5.1"}},{"arch":"x86_64","build_time":"2006-06-02T15:04:05Z","epoch":0,"release":"5.fc30","source":{"license":"MIT","version":"5.0"}}]},{"name":"package6","summary":"pkg6 sum","description":"pkg6 desc","homepage":"https://pkg6.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-07-02T15:04:05Z","epoch":0,"release":"6.fc30","source":{"license":"MIT","version":"6.0"}},{"arch":"x86_64","build_time":"2006-07-03T15:04:05Z","epoch":0,"release":"6.fc30","source":{"license":"MIT","version":"6.1"}}]},{"name":"package7","summary":"pkg7 sum","description":"pkg7 desc","homepage":"https://pkg7.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-08-02T15:04:05Z","epoch":0,"release":"7.fc30","source":{"license":"MIT","version":"7.0"}},{"arch":"x86_64","build_time":"2006-08-03T15:04:05Z","epoch":0,"release":"7.fc30","source":{"license":"MIT","version":"7.1"}}]},{"name":"package8","summary":"pkg8 sum","description":"pkg8 desc","homepage":"https://pkg8.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-09-03T15:04:05Z","epoch":0,"release":"8.fc30","source":{"license":"MIT","version":"8.1"}},{"arch":"x86_64","build_time":"2006-09-02T15:04:05Z","epoch":0,"release":"8.fc30","source":{"license":"MIT","version":"8.0"}}]},{"name":"package9","summary":"pkg9 sum","description":"pkg9 desc","homepage":"https://pkg9.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-10-02T15:04:05Z","epoch":0,"release":"9.fc30","source":{"license":"MIT","version":"9.0"}},{"arch":"x86_64","build_time":"2006-10-03T15:04:05Z","epoch":0,"release":"9.fc30","source":{"license":"MIT","version":"9.1"}}]}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/projects/info/package2*,package16", http.StatusOK, `{"projects":[{"name":"package16","summary":"pkg16 sum","description":"pkg16 desc","homepage":"https://pkg16.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-05-02T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.0"}},{"arch":"x86_64","build_time":"2007-05-03T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.1"}}]},{"name":"package2","summary":"pkg2 sum","description":"pkg2 desc","homepage":"https://pkg2.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-03-02T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.0"}},{"arch":"x86_64","build_time":"2006-03-03T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.1"}}]},{"name":"package20","summary":"pkg20 sum","description":"pkg20 desc","homepage":"https://pkg20.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-09-03T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.1"}},{"arch":"x86_64","build_time":"2007-09-02T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.0"}}]},{"name":"package21","summary":"pkg21 sum","description":"pkg21 desc","homepage":"https://pkg21.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-10-02T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.0"}},{"arch":"x86_64","build_time":"2007-10-03T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.1"}}]}]}`},
		{rpmmd_mock.BadFetch, "/api/v0/projects/info/package2*,package16", http.StatusBadRequest, `{"status":false,"errors":[{"id":"ModulesError","msg":"msg: DNF error occured: FetchError: There was a problem when fetching packages."}]}`},
	}

	for _, c := range cases {
		api, _ := createWeldrAPI(c.Fixture)
		test.TestRoute(t, api, true, "GET", c.Path, ``, c.ExpectedStatus, c.ExpectedJSON)
	}
}

func TestModulesInfo(t *testing.T) {
	var cases = []struct {
		Fixture        rpmmd_mock.FixtureGenerator
		Path           string
		ExpectedStatus int
		ExpectedJSON   string
	}{
		{rpmmd_mock.BaseFixture, "/api/v0/modules/info", http.StatusBadRequest, `{"status":false,"errors":[{"id":"UnknownModule","msg":"No packages specified."}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/modules/info/", http.StatusBadRequest, `{"status":false,"errors":[{"id":"UnknownModule","msg":"No packages specified."}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/modules/info/nonexistingpkg", http.StatusBadRequest, `{"status":false,"errors":[{"id":"UnknownModule","msg":"No packages have been found."}]}`},
		{rpmmd_mock.BadDepsolve, "/api/v0/modules/info/package1", http.StatusBadRequest, `{"status":false,"errors":[{"id":"ModulesError","msg":"Cannot depsolve package package1: DNF error occured: DepsolveError: There was a problem depsolving ['go2rpm']: \n Problem: conflicting requests\n  - nothing provides askalono-cli needed by go2rpm-1-4.fc31.noarch"}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/modules/info/package2*,package16", http.StatusOK, `{"modules":[{"name":"package16","summary":"pkg16 sum","description":"pkg16 desc","homepage":"https://pkg16.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-05-02T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.0"}},{"arch":"x86_64","build_time":"2007-05-03T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package2","summary":"pkg2 sum","description":"pkg2 desc","homepage":"https://pkg2.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-03-02T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.0"}},{"arch":"x86_64","build_time":"2006-03-03T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package20","summary":"pkg20 sum","description":"pkg20 desc","homepage":"https://pkg20.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-09-03T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.1"}},{"arch":"x86_64","build_time":"2007-09-02T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package21","summary":"pkg21 sum","description":"pkg21 desc","homepage":"https://pkg21.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-10-02T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.0"}},{"arch":"x86_64","build_time":"2007-10-03T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]}]}`},
		{rpmmd_mock.BaseFixture, "/api/v0/modules/info/*", http.StatusOK, `{"modules":[{"name":"package0","summary":"pkg0 sum","description":"pkg0 desc","homepage":"https://pkg0.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-01-03T15:04:05Z","epoch":0,"release":"0.fc30","source":{"license":"MIT","version":"0.1"}},{"arch":"x86_64","build_time":"2006-01-02T15:04:05Z","epoch":0,"release":"0.fc30","source":{"license":"MIT","version":"0.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package1","summary":"pkg1 sum","description":"pkg1 desc","homepage":"https://pkg1.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-02-02T15:04:05Z","epoch":0,"release":"1.fc30","source":{"license":"MIT","version":"1.0"}},{"arch":"x86_64","build_time":"2006-02-03T15:04:05Z","epoch":0,"release":"1.fc30","source":{"license":"MIT","version":"1.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package10","summary":"pkg10 sum","description":"pkg10 desc","homepage":"https://pkg10.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-11-02T15:04:05Z","epoch":0,"release":"10.fc30","source":{"license":"MIT","version":"10.0"}},{"arch":"x86_64","build_time":"2006-11-03T15:04:05Z","epoch":0,"release":"10.fc30","source":{"license":"MIT","version":"10.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package11","summary":"pkg11 sum","description":"pkg11 desc","homepage":"https://pkg11.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-12-03T15:04:05Z","epoch":0,"release":"11.fc30","source":{"license":"MIT","version":"11.1"}},{"arch":"x86_64","build_time":"2006-12-02T15:04:05Z","epoch":0,"release":"11.fc30","source":{"license":"MIT","version":"11.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package12","summary":"pkg12 sum","description":"pkg12 desc","homepage":"https://pkg12.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-01-02T15:04:05Z","epoch":0,"release":"12.fc30","source":{"license":"MIT","version":"12.0"}},{"arch":"x86_64","build_time":"2007-01-03T15:04:05Z","epoch":0,"release":"12.fc30","source":{"license":"MIT","version":"12.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package13","summary":"pkg13 sum","description":"pkg13 desc","homepage":"https://pkg13.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-02-02T15:04:05Z","epoch":0,"release":"13.fc30","source":{"license":"MIT","version":"13.0"}},{"arch":"x86_64","build_time":"2007-02-03T15:04:05Z","epoch":0,"release":"13.fc30","source":{"license":"MIT","version":"13.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package14","summary":"pkg14 sum","description":"pkg14 desc","homepage":"https://pkg14.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-03-03T15:04:05Z","epoch":0,"release":"14.fc30","source":{"license":"MIT","version":"14.1"}},{"arch":"x86_64","build_time":"2007-03-02T15:04:05Z","epoch":0,"release":"14.fc30","source":{"license":"MIT","version":"14.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package15","summary":"pkg15 sum","description":"pkg15 desc","homepage":"https://pkg15.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-04-03T15:04:05Z","epoch":0,"release":"15.fc30","source":{"license":"MIT","version":"15.1"}},{"arch":"x86_64","build_time":"2007-04-02T15:04:05Z","epoch":0,"release":"15.fc30","source":{"license":"MIT","version":"15.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package16","summary":"pkg16 sum","description":"pkg16 desc","homepage":"https://pkg16.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-05-02T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.0"}},{"arch":"x86_64","build_time":"2007-05-03T15:04:05Z","epoch":0,"release":"16.fc30","source":{"license":"MIT","version":"16.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package17","summary":"pkg17 sum","description":"pkg17 desc","homepage":"https://pkg17.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-06-03T15:04:05Z","epoch":0,"release":"17.fc30","source":{"license":"MIT","version":"17.1"}},{"arch":"x86_64","build_time":"2007-06-02T15:04:05Z","epoch":0,"release":"17.fc30","source":{"license":"MIT","version":"17.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package18","summary":"pkg18 sum","description":"pkg18 desc","homepage":"https://pkg18.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-07-02T15:04:05Z","epoch":0,"release":"18.fc30","source":{"license":"MIT","version":"18.0"}},{"arch":"x86_64","build_time":"2007-07-03T15:04:05Z","epoch":0,"release":"18.fc30","source":{"license":"MIT","version":"18.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package19","summary":"pkg19 sum","description":"pkg19 desc","homepage":"https://pkg19.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-08-03T15:04:05Z","epoch":0,"release":"19.fc30","source":{"license":"MIT","version":"19.1"}},{"arch":"x86_64","build_time":"2007-08-02T15:04:05Z","epoch":0,"release":"19.fc30","source":{"license":"MIT","version":"19.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package2","summary":"pkg2 sum","description":"pkg2 desc","homepage":"https://pkg2.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-03-02T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.0"}},{"arch":"x86_64","build_time":"2006-03-03T15:04:05Z","epoch":0,"release":"2.fc30","source":{"license":"MIT","version":"2.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package20","summary":"pkg20 sum","description":"pkg20 desc","homepage":"https://pkg20.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-09-03T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.1"}},{"arch":"x86_64","build_time":"2007-09-02T15:04:05Z","epoch":0,"release":"20.fc30","source":{"license":"MIT","version":"20.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package21","summary":"pkg21 sum","description":"pkg21 desc","homepage":"https://pkg21.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2007-10-02T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.0"}},{"arch":"x86_64","build_time":"2007-10-03T15:04:05Z","epoch":0,"release":"21.fc30","source":{"license":"MIT","version":"21.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package3","summary":"pkg3 sum","description":"pkg3 desc","homepage":"https://pkg3.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-04-03T15:04:05Z","epoch":0,"release":"3.fc30","source":{"license":"MIT","version":"3.1"}},{"arch":"x86_64","build_time":"2006-04-02T15:04:05Z","epoch":0,"release":"3.fc30","source":{"license":"MIT","version":"3.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package4","summary":"pkg4 sum","description":"pkg4 desc","homepage":"https://pkg4.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-05-03T15:04:05Z","epoch":0,"release":"4.fc30","source":{"license":"MIT","version":"4.1"}},{"arch":"x86_64","build_time":"2006-05-02T15:04:05Z","epoch":0,"release":"4.fc30","source":{"license":"MIT","version":"4.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package5","summary":"pkg5 sum","description":"pkg5 desc","homepage":"https://pkg5.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-06-03T15:04:05Z","epoch":0,"release":"5.fc30","source":{"license":"MIT","version":"5.1"}},{"arch":"x86_64","build_time":"2006-06-02T15:04:05Z","epoch":0,"release":"5.fc30","source":{"license":"MIT","version":"5.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package6","summary":"pkg6 sum","description":"pkg6 desc","homepage":"https://pkg6.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-07-02T15:04:05Z","epoch":0,"release":"6.fc30","source":{"license":"MIT","version":"6.0"}},{"arch":"x86_64","build_time":"2006-07-03T15:04:05Z","epoch":0,"release":"6.fc30","source":{"license":"MIT","version":"6.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package7","summary":"pkg7 sum","description":"pkg7 desc","homepage":"https://pkg7.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-08-02T15:04:05Z","epoch":0,"release":"7.fc30","source":{"license":"MIT","version":"7.0"}},{"arch":"x86_64","build_time":"2006-08-03T15:04:05Z","epoch":0,"release":"7.fc30","source":{"license":"MIT","version":"7.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package8","summary":"pkg8 sum","description":"pkg8 desc","homepage":"https://pkg8.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-09-03T15:04:05Z","epoch":0,"release":"8.fc30","source":{"license":"MIT","version":"8.1"}},{"arch":"x86_64","build_time":"2006-09-02T15:04:05Z","epoch":0,"release":"8.fc30","source":{"license":"MIT","version":"8.0"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]},{"name":"package9","summary":"pkg9 sum","description":"pkg9 desc","homepage":"https://pkg9.example.com","upstream_vcs":"","builds":[{"arch":"x86_64","build_time":"2006-10-02T15:04:05Z","epoch":0,"release":"9.fc30","source":{"license":"MIT","version":"9.0"}},{"arch":"x86_64","build_time":"2006-10-03T15:04:05Z","epoch":0,"release":"9.fc30","source":{"license":"MIT","version":"9.1"}}],"dependencies":[{"name":"dep-package1","epoch":0,"version":"1.33","release":"2.fc30","arch":"x86_64"},{"name":"dep-package2","epoch":0,"version":"2.9","release":"1.fc30","arch":"x86_64"}]}]}`},
		{rpmmd_mock.BadFetch, "/api/v0/modules/info/package2*,package16", http.StatusBadRequest, `{"status":false,"errors":[{"id":"ModulesError","msg":"msg: DNF error occured: FetchError: There was a problem when fetching packages."}]}`},
	}

	for _, c := range cases {
		api, _ := createWeldrAPI(c.Fixture)
		test.TestRoute(t, api, true, "GET", c.Path, ``, c.ExpectedStatus, c.ExpectedJSON)
	}
}

func TestProjectsList(t *testing.T) {
	var cases = []struct {
		Fixture        rpmmd_mock.FixtureGenerator
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
		test.TestRoute(t, api, true, "GET", c.Path, ``, c.ExpectedStatus, c.ExpectedJSON)
	}
}

func TestModulesList(t *testing.T) {
	var cases = []struct {
		Fixture        rpmmd_mock.FixtureGenerator
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
		test.TestRoute(t, api, true, "GET", c.Path, ``, c.ExpectedStatus, c.ExpectedJSON)
	}
}
