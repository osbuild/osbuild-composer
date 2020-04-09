// Package client contains functions for communicating with the API server
// Copyright (C) 2020 by Red Hat, Inc.
package client

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequest(t *testing.T) {
	// Make a request to the status route
	resp, err := Request(testState.socket, "GET", "/api/status", "", map[string]string{})
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	// Make a request to a bad route
	resp, err = Request(testState.socket, "GET", "/invalidroute", "", map[string]string{})
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Test that apiError returns an error response
	_, err = apiError(resp)
	require.NoError(t, err)

	// Make a request with a bad offset to trigger a JSON response with Status set to 400
	resp, err = Request(testState.socket, "GET", "/api/v0/blueprints/list?offset=bad", "", map[string]string{})
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAPIErrorMsg(t *testing.T) {
	err := APIErrorMsg{ID: "ERROR_ID", Msg: "Meaningful error message"}
	require.Equal(t, "ERROR_ID: Meaningful error message", err.String())
}

func TestAPIResponse(t *testing.T) {
	resp := APIResponse{Status: true}
	require.Equal(t, "", resp.String())

	resp = APIResponse{Status: false,
		Errors: []APIErrorMsg{
			{ID: "ONE_ERROR", Msg: "First message"},
			{ID: "TWO_ERROR", Msg: "Second message"}},
	}
	require.Equal(t, "ONE_ERROR: First message", resp.String())
	require.ElementsMatch(t, []string{
		"ONE_ERROR: First message",
		"TWO_ERROR: Second message"}, resp.AllErrors())
}

func TestGetRaw(t *testing.T) {
	// Get raw data
	b, resp, err := GetRaw(testState.socket, "GET", "/api/status")
	require.NoError(t, err)
	require.Nil(t, resp)
	require.Greater(t, len(b), 0)

	// Get an API error
	b, resp, err = GetRaw(testState.socket, "GET", "/api/v0/blueprints/list?offset=bad")
	require.NoError(t, err)
	require.NotNilf(t, resp, "GetRaw bad request did not return an error: %v", b)
	require.False(t, resp.Status)
	require.GreaterOrEqual(t, len(resp.AllErrors()), 1)
	require.Equal(t, "BadLimitOrOffset", resp.Errors[0].ID)
}

func TestGetJSONAll(t *testing.T) {
	// Get all the projects
	b, resp, err := GetJSONAll(testState.socket, "/api/v0/projects/list")
	require.NoError(t, err)
	require.Nil(t, resp)
	require.GreaterOrEqualf(t, len(b), 100, "GetJSONAll response is too short: %#v", b)

	// Run it on a route that doesn't support offset/limit
	_, _, err = GetJSONAll(testState.socket, "/api/status")
	require.EqualError(t, err, "Response is missing the total value")
}

func TestPostRaw(t *testing.T) {
	// There are no routes that accept raw POST w/o Content-Type so this ends up testing the error path
	b, resp, err := PostRaw(testState.socket, "/api/v0/blueprints/new", "nobody", nil)
	require.NoError(t, err)
	require.NotNilf(t, resp, "PostRaw bad request did not return an error: %v", b)
	require.False(t, resp.Status)
	require.GreaterOrEqualf(t, len(resp.AllErrors()), 1, "GetRaw error did not return error message: %#v", resp)
	require.Equalf(t, "BlueprintsError", resp.Errors[0].ID, "GetRaw error ID is not BlueprintsError: %#v", resp)
}

func TestPostTOML(t *testing.T) {
	blueprint := `name = "test-blueprint"
				  description = "TOML test blueprint"
				  version = "0.0.1"`
	b, resp, err := PostTOML(testState.socket, "/api/v0/blueprints/new", blueprint)
	require.NoError(t, err)
	require.Nil(t, resp)
	require.Contains(t, string(b), "true")
}

func TestPostJSON(t *testing.T) {
	blueprint := `{"name": "test-blueprint",
				   "description": "JSON test blueprint",
				   "version": "0.0.1"}`
	b, resp, err := PostJSON(testState.socket, "/api/v0/blueprints/new", blueprint)
	require.NoError(t, err)
	require.Nil(t, resp)
	require.Contains(t, string(b), "true")
}
