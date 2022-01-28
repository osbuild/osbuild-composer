// Package client - compose contains functions for the compose API
// Copyright (C) 2020 by Red Hat, Inc.
package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/osbuild/osbuild-composer/internal/weldr"
)

// PostComposeV0 sends a JSON compose string to the API
// and returns an APIResponse
func PostComposeV0(socket *http.Client, compose string) (*APIResponse, error) {
	body, resp, err := PostJSON(socket, "/api/v0/compose", compose)
	if resp != nil || err != nil {
		return resp, err
	}
	return NewAPIResponse(body)
}

// NewComposeResponseV0 converts the response body to a status response
func NewComposeResponseV0(body []byte) (*weldr.ComposeResponseV0, error) {
	var response weldr.ComposeResponseV0
	err := json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// GetFinishedComposesV0 returns a list of the finished composes
func GetFinishedComposesV0(socket *http.Client) ([]weldr.ComposeEntryV0, *APIResponse, error) {
	body, resp, err := GetRaw(socket, "GET", "/api/v0/compose/finished")
	if resp != nil || err != nil {
		return []weldr.ComposeEntryV0{}, resp, err
	}
	var finished weldr.ComposeFinishedResponseV0
	err = json.Unmarshal(body, &finished)
	if err != nil {
		return []weldr.ComposeEntryV0{}, nil, err
	}
	return finished.Finished, nil, nil
}

// GetFailedComposesV0 returns a list of the failed composes
func GetFailedComposesV0(socket *http.Client) ([]weldr.ComposeEntryV0, *APIResponse, error) {
	body, resp, err := GetRaw(socket, "GET", "/api/v0/compose/failed")
	if resp != nil || err != nil {
		return []weldr.ComposeEntryV0{}, resp, err
	}
	var failed weldr.ComposeFailedResponseV0
	err = json.Unmarshal(body, &failed)
	if err != nil {
		return []weldr.ComposeEntryV0{}, nil, err
	}
	return failed.Failed, nil, nil
}

// GetComposeStatusV0 returns a list of composes matching the optional filter parameters
func GetComposeStatusV0(socket *http.Client, uuids, blueprint, status, composeType string) ([]weldr.ComposeEntryV0, *APIResponse, error) {
	// Build the query string
	route := "/api/v0/compose/status/" + uuids

	params := url.Values{}
	if len(blueprint) > 0 {
		params.Add("blueprint", blueprint)
	}
	if len(status) > 0 {
		params.Add("status", status)
	}
	if len(composeType) > 0 {
		params.Add("type", composeType)
	}

	if len(params) > 0 {
		route = route + "?" + params.Encode()
	}

	body, resp, err := GetRaw(socket, "GET", route)
	if resp != nil || err != nil {
		return []weldr.ComposeEntryV0{}, resp, err
	}
	var composes weldr.ComposeStatusResponseV0
	err = json.Unmarshal(body, &composes)
	if err != nil {
		return []weldr.ComposeEntryV0{}, nil, err
	}
	return composes.UUIDs, nil, nil
}

// GetComposeTypesV0 returns a list of the failed composes
func GetComposesTypesV0(socket *http.Client) ([]weldr.ComposeTypeV0, *APIResponse, error) {
	body, resp, err := GetRaw(socket, "GET", "/api/v0/compose/types")
	if resp != nil || err != nil {
		return []weldr.ComposeTypeV0{}, resp, err
	}
	var composeTypes weldr.ComposeTypesResponseV0
	err = json.Unmarshal(body, &composeTypes)
	if err != nil {
		return []weldr.ComposeTypeV0{}, nil, err
	}
	return composeTypes.Types, nil, nil
}

// CancelComposeV0 cancels one composes based on the uuid
func CancelComposeV0(socket *http.Client, uuid string) (weldr.CancelComposeStatusV0, *APIResponse, error) {
	body, resp, err := DeleteRaw(socket, "/api/v0/compose/cancel/"+uuid)
	if resp != nil || err != nil {
		return weldr.CancelComposeStatusV0{}, resp, err
	}
	var status weldr.CancelComposeStatusV0
	err = json.Unmarshal(body, &status)
	if err != nil {
		return weldr.CancelComposeStatusV0{}, nil, err
	}
	return status, nil, nil
}

// DeleteComposeV0 deletes one or more composes based on their uuid
func DeleteComposeV0(socket *http.Client, uuids string) (weldr.DeleteComposeResponseV0, *APIResponse, error) {
	body, resp, err := DeleteRaw(socket, "/api/v0/compose/delete/"+uuids)
	if resp != nil || err != nil {
		return weldr.DeleteComposeResponseV0{}, resp, err
	}
	var deleteResponse weldr.DeleteComposeResponseV0
	err = json.Unmarshal(body, &deleteResponse)
	if err != nil {
		return weldr.DeleteComposeResponseV0{}, nil, err
	}
	return deleteResponse, nil, nil
}

// GetComposeInfoV0 returns detailed information about the selected compose
func GetComposeInfoV0(socket *http.Client, uuid string) (weldr.ComposeInfoResponseV0, *APIResponse, error) {
	body, resp, err := GetRaw(socket, "GET", "/api/v0/compose/info/"+uuid)
	if resp != nil || err != nil {
		return weldr.ComposeInfoResponseV0{}, resp, err
	}
	var info weldr.ComposeInfoResponseV0
	err = json.Unmarshal(body, &info)
	if err != nil {
		return weldr.ComposeInfoResponseV0{}, nil, err
	}
	return info, nil, nil
}

// GetComposeQueueV0 returns the list of composes in the queue
func GetComposeQueueV0(socket *http.Client) (weldr.ComposeQueueResponseV0, *APIResponse, error) {
	body, resp, err := GetRaw(socket, "GET", "/api/v0/compose/queue")
	if resp != nil || err != nil {
		return weldr.ComposeQueueResponseV0{}, resp, err
	}
	var queue weldr.ComposeQueueResponseV0
	err = json.Unmarshal(body, &queue)
	if err != nil {
		return weldr.ComposeQueueResponseV0{}, nil, err
	}
	return queue, nil, nil
}

// Test compose metadata for unknown uuid

// Test compose results for unknown uuid

// WriteComposeImageV0 requests the image for a compose and writes it to an io.Writer
func WriteComposeImageV0(socket *http.Client, w io.Writer, uuid string) (*APIResponse, error) {
	body, resp, err := GetRawBody(socket, "GET", "/api/v0/compose/image/"+uuid)
	if resp != nil || err != nil {
		return resp, err
	}
	_, err = io.Copy(w, body)
	body.Close()

	return nil, err
}

// WriteComposeLogsV0 requests the logs for a compose and writes it to an io.Writer
func WriteComposeLogsV0(socket *http.Client, w io.Writer, uuid string) (*APIResponse, error) {
	body, resp, err := GetRawBody(socket, "GET", "/api/v0/compose/logs/"+uuid)
	if resp != nil || err != nil {
		return resp, err
	}
	_, err = io.Copy(w, body)
	body.Close()

	return nil, err
}

// WriteComposeLogV0 requests the log for a compose and writes it to an io.Writer
func WriteComposeLogV0(socket *http.Client, w io.Writer, uuid string) (*APIResponse, error) {
	body, resp, err := GetRawBody(socket, "GET", "/api/v0/compose/log/"+uuid)
	if resp != nil || err != nil {
		return resp, err
	}
	_, err = io.Copy(w, body)
	body.Close()

	return nil, err
}

// WriteComposeMetadataV0 requests the metadata for a compose and writes it to an io.Writer
func WriteComposeMetadataV0(socket *http.Client, w io.Writer, uuid string) (*APIResponse, error) {
	body, resp, err := GetRawBody(socket, "GET", "/api/v0/compose/metadata/"+uuid)
	if resp != nil || err != nil {
		return resp, err
	}
	_, err = io.Copy(w, body)
	body.Close()

	return nil, err
}

// WriteComposeResultsV0 requests the results for a compose and writes it to an io.Writer
func WriteComposeResultsV0(socket *http.Client, w io.Writer, uuid string) (*APIResponse, error) {
	body, resp, err := GetRawBody(socket, "GET", "/api/v0/compose/metadata/"+uuid)
	if resp != nil || err != nil {
		return resp, err
	}
	_, err = io.Copy(w, body)
	body.Close()

	return nil, err
}
