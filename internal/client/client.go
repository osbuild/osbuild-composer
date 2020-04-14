// Package client contains functions for communicating with the API server
// Copyright (C) 2020 by Red Hat, Inc.
package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

// Request handles sending the request, handling errors, returning the response
// socket is the path to a Unix Domain socket
// path is the full URL path, including query strings
// body is the data to send with POST
// headers is a map of header:value to add to the request
//
// If it is successful a http.Response will be returned. If there is an error, the response will be
// nil and error will be returned.
func Request(socket *http.Client, method, path, body string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(method, "http://localhost"+path, bytes.NewReader([]byte(body)))
	if err != nil {
		return nil, err
	}

	for h, v := range headers {
		req.Header.Set(h, v)
	}

	resp, err := socket.Do(req)
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

// APIResponse is returned by some requests to indicate success or failure.
// It is always returned when the status code is 400, indicating some kind of error with the request.
// If Status is true the Errors list will not be included or will be empty.
// When Status is false it will include at least one APIErrorMsg with details about the error.
type APIResponse struct {
	Status bool          `json:"status"`
	Errors []APIErrorMsg `json:"errors,omitempty"`
}

// String returns the description of the first error, if there is one
func (r *APIResponse) String() string {
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

// NewAPIResponse converts the response body to a status response
func NewAPIResponse(body []byte) (*APIResponse, error) {
	var status APIResponse
	err := json.Unmarshal(body, &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

// apiError converts an API error 400 JSON to a status response
//
// The response body should alway be of the form:
//     {"status": false, "errors": [{"id": ERROR_ID, "msg": ERROR_MESSAGE}, ...]}
func apiError(resp *http.Response) (*APIResponse, error) {
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return NewAPIResponse(body)
}

// GetRawBody returns the resp.Body io.ReadCloser to the caller
// NOTE: The caller is responsible for closing the Body when finished
func GetRawBody(socket *http.Client, method, path string) (io.ReadCloser, *APIResponse, error) {
	resp, err := Request(socket, method, path, "", map[string]string{})
	if err != nil {
		return nil, nil, err
	}

	// Convert the API's JSON error response to an error type and return it
	// lorax-composer (wrongly) returns 404 for some of its json responses
	if resp.StatusCode == 400 || resp.StatusCode == 404 {
		apiResponse, err := apiError(resp)
		return nil, apiResponse, err
	}
	return resp.Body, nil, nil
}

// GetRaw returns raw data from a GET request
// Errors from the API are returned as an APIResponse, client errors are returned as error
func GetRaw(socket *http.Client, method, path string) ([]byte, *APIResponse, error) {
	body, resp, err := GetRawBody(socket, method, path)
	if err != nil || resp != nil {
		return nil, resp, err
	}
	defer body.Close()

	bodyBytes, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, nil, err
	}

	return bodyBytes, nil, nil
}

// GetJSONAll returns all JSON results from a GET request using offset/limit
// This function makes 2 requests, the first with limit=0 to get the total number of results,
// and then with limit=TOTAL to fetch all of the results.
// The path passed to GetJSONAll should not include the limit or offset query parameters
// Errors from the API are returned as an APIResponse, client errors are returned as error
func GetJSONAll(socket *http.Client, path string) ([]byte, *APIResponse, error) {
	body, api, err := GetRaw(socket, "GET", path+"?limit=0")
	if api != nil || err != nil {
		return nil, api, err
	}

	// We just want the total
	var j interface{}
	err = json.Unmarshal(body, &j)
	if err != nil {
		return nil, nil, err
	}
	m := j.(map[string]interface{})
	var v interface{}
	var ok bool
	if v, ok = m["total"]; !ok {
		return nil, nil, errors.New("Response is missing the total value")
	}

	switch total := v.(type) {
	case float64:
		allResults := fmt.Sprintf("%s?limit=%v", path, total)
		return GetRaw(socket, "GET", allResults)
	}
	return nil, nil, errors.New("Response 'total' is not a float64")
}

// PostRaw sends a POST with raw data and returns the raw response body
// Errors from the API are returned as an APIResponse, client errors are returned as error
func PostRaw(socket *http.Client, path, body string, headers map[string]string) ([]byte, *APIResponse, error) {
	resp, err := Request(socket, "POST", path, body, headers)
	if err != nil {
		return nil, nil, err
	}

	// Convert the API's JSON error response to an APIResponse
	if resp.StatusCode == 400 {
		apiResponse, err := apiError(resp)
		return nil, apiResponse, err
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	return responseBody, nil, nil
}

// PostTOML sends a POST with TOML data and the Content-Type header set to "text/x-toml"
// Errors from the API are returned as an APIResponse, client errors are returned as error
func PostTOML(socket *http.Client, path, body string) ([]byte, *APIResponse, error) {
	headers := map[string]string{"Content-Type": "text/x-toml"}
	return PostRaw(socket, path, body, headers)
}

// PostJSON sends a POST with JSON data and the Content-Type header set to "application/json"
// Errors from the API are returned as an APIResponse, client errors are returned as error
func PostJSON(socket *http.Client, path, body string) ([]byte, *APIResponse, error) {
	headers := map[string]string{"Content-Type": "application/json"}
	return PostRaw(socket, path, body, headers)
}

// DeleteRaw sends a DELETE request
// Errors from the API are returned as an APIResponse, client errors are returned as error
func DeleteRaw(socket *http.Client, path string) ([]byte, *APIResponse, error) {
	resp, err := Request(socket, "DELETE", path, "", nil)
	if err != nil {
		return nil, nil, err
	}

	// Convert the API's JSON error response to an APIResponse
	if resp.StatusCode == 400 {
		apiResponse, err := apiError(resp)
		return nil, apiResponse, err
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	return responseBody, nil, nil
}
