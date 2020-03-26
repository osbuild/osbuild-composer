// Package client contains functions for communicating with the API server
// Copyright (C) 2020 by Red Hat, Inc.
package client

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRequest(t *testing.T) {
	// Make a request to the status route
	resp, err := Request(testState.socket, "GET", "/api/status", "", map[string]string{})
	if err != nil {
		t.Fatalf("Request good route failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Request good route: %d != 200", resp.StatusCode)
	}

	// Make a request to a bad route
	resp, err = Request(testState.socket, "GET", "/invalidroute", "", map[string]string{})
	if err != nil {
		t.Fatalf("Request bad route failed: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Request bad route: %d != 404", resp.StatusCode)
	}

	// Test that apiError returns an error when trying to parse non-JSON response
	_, err = apiError(resp)
	if err == nil {
		t.Fatalf("apiError of a 404 response did not return an error: %#v", resp)
	}

	// Make a request with a bad offset to trigger a JSON response with Status set to 400
	resp, err = Request(testState.socket, "GET", "/api/v0/blueprints/list?offset=bad", "", map[string]string{})
	if err != nil {
		t.Fatalf("Request bad offset failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Request bad offset: %d != 400", resp.StatusCode)
	}
}

func TestAPIErrorMsg(t *testing.T) {
	err := APIErrorMsg{ID: "ERROR_ID", Msg: "Meaningful error message"}
	if diff := cmp.Diff(err.String(), "ERROR_ID: Meaningful error message"); diff != "" {
		t.Fatalf("APIErrorMsg: %s", diff)
	}
}

func TestAPIResponse(t *testing.T) {
	resp := APIResponse{Status: true}
	if resp.String() != "" {
		t.Fatalf("Empty APIResponse Errors doesn't return empty string: %v", resp.String())
	}

	resp = APIResponse{Status: false,
		Errors: []APIErrorMsg{
			{ID: "ONE_ERROR", Msg: "First message"},
			{ID: "TWO_ERROR", Msg: "Second message"}},
	}
	if diff := cmp.Diff(resp.String(), "ONE_ERROR: First message"); diff != "" {
		t.Fatalf("APIResponse.Error: %s", diff)
	}
	if diff := cmp.Diff(resp.AllErrors(), []string{"ONE_ERROR: First message", "TWO_ERROR: Second message"}); diff != "" {
		t.Fatalf("APIErrorMsg: %s", diff)
	}
}

func TestGetRaw(t *testing.T) {
	// Get raw data
	b, resp, err := GetRaw(testState.socket, "GET", "/api/status")
	if err != nil {
		t.Fatalf("GetRaw failed with a client error: %v", err)
	}
	if resp != nil {
		t.Fatalf("GetRaw request failed: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("GetRaw returned an empty string")
	}
	// Get an API error
	b, resp, err = GetRaw(testState.socket, "GET", "/api/v0/blueprints/list?offset=bad")
	if err != nil {
		t.Fatalf("GetRaw bad request failed with a client error: %v", err)
	}
	if resp == nil {
		t.Fatalf("GetRaw bad request did not return an error: %v", b)
	}
	if resp.Status != false {
		t.Fatalf("Status != false: %#v", resp)
	}
	if len(resp.AllErrors()) < 1 {
		t.Fatalf("GetRaw error did not return error message: %#v", resp)
	} else if resp.Errors[0].ID != "BadLimitOrOffset" {
		t.Fatalf("GetRaw error ID is not BadLimitOrOffset: %#v", resp)
	}
}

func TestGetJSONAll(t *testing.T) {
	// Get all the projects
	b, resp, err := GetJSONAll(testState.socket, "/api/v0/projects/list")
	if err != nil {
		t.Fatalf("GetJSONAll failed with a client error: %v", err)
	}
	if resp != nil {
		t.Fatalf("GetJSONAll request failed: %v", resp)
	}
	if len(b) < 100 {
		t.Fatalf("GetJSONAll response is too short: %#v", b)
	}

	// Run it on a route that doesn't support offset/limit
	b, resp, err = GetJSONAll(testState.socket, "/api/status")
	if err == nil {
		t.Fatalf("GetJSONAll bad route failed: %v", b)
	}
	if err.Error() != "Response is missing the total value" {
		t.Fatalf("GetJSONAll bad route has unexpected total value: %v", resp)
	}
}

func TestPostRaw(t *testing.T) {
	// There are no routes that accept raw POST w/o Content-Type so this ends up testing the error path
	b, resp, err := PostRaw(testState.socket, "/api/v0/blueprints/new", "nobody", nil)
	if err != nil {
		t.Fatalf("PostRaw bad request failed with a client error: %v", err)
	}
	if resp == nil {
		t.Fatalf("PostRaw bad request did not return an error: %v", b)
	}
	if resp.Status != false {
		t.Fatalf("PostRaw bad request status != false: %#v", resp)
	}
	if len(resp.AllErrors()) < 1 {
		t.Fatalf("GetRaw error did not return error message: %#v", resp)
	} else if resp.Errors[0].ID != "BlueprintsError" {
		t.Fatalf("GetRaw error ID is not BlueprintsError: %#v", resp)
	}
}

func TestPostTOML(t *testing.T) {
	blueprint := `name = "test-blueprint"
				  description = "TOML test blueprint"
				  version = "0.0.1"`
	b, resp, err := PostTOML(testState.socket, "/api/v0/blueprints/new", blueprint)
	if err != nil {
		t.Fatalf("PostTOML client failed: %v", err)
	}
	if resp != nil {
		t.Fatalf("PostTOML request failed: %v", resp)
	}
	if !strings.Contains(string(b), "true") {
		t.Fatalf("PostTOML failed: %#v", string(b))
	}
}

func TestPostJSON(t *testing.T) {
	blueprint := `{"name": "test-blueprint",
				   "description": "JSON test blueprint",
				   "version": "0.0.1"}`
	b, resp, err := PostJSON(testState.socket, "/api/v0/blueprints/new", blueprint)
	if err != nil {
		t.Fatalf("PostJSON client failed: %v", err)
	}
	if resp != nil {
		t.Fatalf("PostJSON request failed: %v", resp)
	}
	if !strings.Contains(string(b), "true") {
		t.Fatalf("PostJSON failed: %#v", string(b))
	}
}
