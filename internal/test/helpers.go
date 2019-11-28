package test

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

	"github.com/google/go-cmp/cmp"
)

type API interface {
	ServeHTTP(writer http.ResponseWriter, request *http.Request)
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

func internalRequest(api API, method, path, body string) *http.Response {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	api.ServeHTTP(resp, req)

	return resp.Result()
}

func SendHTTP(api API, external bool, method, path, body string) *http.Response {
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

func TestRoute(t *testing.T, api API, external bool, method, path, body string, expectedStatus int, expectedJSON string, ignoreFields ...string) {
	resp := SendHTTP(api, external, method, path, body)
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

	if diff := cmp.Diff(expected, reply); diff != "" {
		t.Errorf("%s: reply != expected:\n   reply: %s\nexpected: %s\ndiff: %s", path, strings.TrimSpace(string(replyJSON)), expectedJSON, diff)
		return
	}
}
