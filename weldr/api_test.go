package weldr_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"osbuild-composer/rpmmd"
	"osbuild-composer/weldr"
)

var repo = rpmmd.RepoConfig{
	Id:      "test",
	Name:    "Test",
	BaseURL: "http://example.com/test/os",
}

var packages = rpmmd.PackageList {
	{ Name: "package1" },
	{ Name: "package2" },
}

func TestAPI(t *testing.T) {
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

	for _, c:= range cases {
		req := httptest.NewRequest("GET", c.Path, nil)
		resp := httptest.NewRecorder()

		api := weldr.New(repo, packages, nil)
		api.ServeHTTP(resp, req)

		if resp.Code != c.ExpectedStatus {
			t.Errorf("%s: unexpected status code: %v", c.Path, resp.Code)
			continue
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Errorf("%s: could not read reponse body: %v", c.Path, err)
			continue
		}

		if c.ExpectedJSON == "" {
			if len(body) != 0 {
				t.Errorf("%s: expected no response body, but got:\n%s", c.Path, body)
			}
			continue
		}

		var reply, expected interface{}
		err = json.Unmarshal(body, &reply)
		if err != nil {
			t.Errorf("%s: %v\n%s", c.Path, err, string(body))
			continue
		}

		if c.ExpectedJSON == "*" {
			continue
		}

		err = json.Unmarshal([]byte(c.ExpectedJSON), &expected)
		if err != nil {
			t.Errorf("%s: expected JSON is invalid: %v", c.Path, err)
			continue
		}

		if !reflect.DeepEqual(reply, expected) {
			t.Errorf("%s: reply != expected:\n   reply: %s\nexpected: %s", c.Path, strings.TrimSpace(string(body)), c.ExpectedJSON)
			continue
		}
	}
}
