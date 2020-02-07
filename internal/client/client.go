// Package client contains functions for communicating with the API server
// Copyright (C) 2020 by Red Hat, Inc.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

// Request handles sending the request, handling errors, returning the response
// socket is the path to a Unix Domain socket
// path is the full URL path, including query strings
// body is the data to send with POST
// headers is a map of header:value to add to the request
//
// If it is successful a http.Response will be returned. If there is an error, the response will be
// nil and error will be returned.
func Request(socket, method, path, body string, headers map[string]string) (*http.Response, error) {
	client := http.Client{
		// TODO This may be too short/simple for downloading images
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socket)
			},
		},
	}

	req, err := http.NewRequest(method, "http://localhost"+path, bytes.NewReader([]byte(body)))
	if err != nil {
		return nil, err
	}

	for h, v := range headers {
		req.Header.Set(h, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// APIErrorMsg is an individual API error with an ID and a message string
type APIErrorMsg struct {
	ID  string `json:"id"`
	Msg string `json:"msg"`
}

// String returns the error id and message as a string
func (r *APIErrorMsg) String() string {
	return fmt.Sprintf("%s: %s", r.ID, r.Msg)
}

// APIResponse is returned with status code 400 and may contain a list of errors
// If Status is true the Errors list will not be included or will be empty.
// When Status is false it will include at least one APIErrorMsg with details about the error.
// It also implements the error interface so that it can be used in place of error
type APIResponse struct {
	Status bool          `json:"status"`
	Errors []APIErrorMsg `json:"errors,omitempty"`
}

// Error returns the description of the first error
func (r *APIResponse) Error() string {
	if len(r.Errors) == 0 {
		return ""
	}
	return r.Errors[0].String()
}

// AllErrors returns a list of error description strings
func (r *APIResponse) AllErrors() (all []string) {
	for i := range r.Errors {
		all = append(all, r.Errors[i].String())
	}
	return all
}

// clientError converts an error into an APIResponse with ID set to ClientError
// This is used to return golang function errors to callers of the client functions
func clientError(err error) *APIResponse {
	return &APIResponse{
		Status: false,
		Errors: []APIErrorMsg{{ID: "ClientError", Msg: err.Error()}},
	}
}

// NewAPIResponse converts the response body to a status response
func NewAPIResponse(body []byte) *APIResponse {
	var status APIResponse
	err := json.Unmarshal(body, &status)
	if err != nil {
		return clientError(err)
	}
	return &status
}

// apiError converts an API error 400 JSON to a status response
//
// The response body should alway be of the form:
//     {"status": false, "errors": [{"id": ERROR_ID, "msg": ERROR_MESSAGE}, ...]}
func apiError(resp *http.Response) *APIResponse {
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return clientError(err)
	}
	return NewAPIResponse(body)
}

// GetRaw returns raw data from a GET request
// Errors from the client and from the API are returned as an APIResponse
func GetRaw(socket, method, path string) ([]byte, *APIResponse) {
	resp, err := Request(socket, method, path, "", map[string]string{})
	if err != nil {
		return nil, clientError(err)
	}

	// Convert the API's JSON error response to an error type and return it
	if resp.StatusCode == 400 {
		return nil, apiError(resp)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, clientError(err)
	}

	return body, nil
}

// GetJSONAll returns all JSON results from a GET request using offset/limit
// This function makes 2 requests, the first with limit=0 to get the total number of results,
// and then with limit=TOTAL to fetch all of the results.
// The path passed to GetJSONAll should not include the limit or offset query parameters
// Errors from the client and from the API are returned as an APIResponse
func GetJSONAll(socket, path string) ([]byte, *APIResponse) {
	body, err := GetRaw(socket, "GET", path+"?limit=0")
	if err != nil {
		return nil, err
	}

	// We just want the total
	var j interface{}
	jerr := json.Unmarshal(body, &j)
	if jerr != nil {
		return nil, clientError(jerr)
	}
	m := j.(map[string]interface{})
	var v interface{}
	var ok bool
	if v, ok = m["total"]; !ok {
		return nil, clientError(errors.New("Response is missing the total value"))
	}

	switch total := v.(type) {
	case float64:
		allResults := fmt.Sprintf("%s?limit=%v", path, total)
		return GetRaw(socket, "GET", allResults)
	}
	return nil, clientError(errors.New("Response 'total' is wrong type"))
}

// PostRaw sends a POST with raw data and returns the raw response body
// Errors from the client and from the API are returned as an APIResponse
func PostRaw(socket, path, body string, headers map[string]string) ([]byte, *APIResponse) {
	resp, err := Request(socket, "POST", path, body, headers)
	if err != nil {
		return nil, clientError(err)
	}

	// Convert the API's JSON error response to an error type and return it
	if resp.StatusCode == 400 {
		return nil, apiError(resp)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, clientError(err)
	}

	return responseBody, nil
}

// PostTOML sends a POST with TOML data and the Content-Type header set to "text/x-toml"
// It returns the raw response data or errors as an APIResponse
func PostTOML(socket, path, body string) ([]byte, *APIResponse) {
	headers := map[string]string{"Content-Type": "text/x-toml"}
	return PostRaw(socket, path, body, headers)
}

// PostJSON sends a POST with JSON data and the Content-Type header set to "application/json"
// It returns the raw response data or errors as an APIResponse
func PostJSON(socket, path, body string) ([]byte, *APIResponse) {
	headers := map[string]string{"Content-Type": "application/json"}
	return PostRaw(socket, path, body, headers)
}

// DeleteRaw sends a DELETE request
// It returns the raw response data or errors as an APIResponse
func DeleteRaw(socket, path string) ([]byte, *APIResponse) {
	resp, err := Request(socket, "DELETE", path, "", nil)
	if err != nil {
		return nil, clientError(err)
	}

	// Convert the API's JSON error response to an error type and return it
	if resp.StatusCode == 400 {
		return nil, apiError(resp)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, clientError(err)
	}

	return responseBody, nil
}
