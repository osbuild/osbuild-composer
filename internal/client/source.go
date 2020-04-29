// Package client - source contains functions for the source API
// Copyright (C) 2020 by Red Hat, Inc.
package client

import (
	"encoding/json"
	//	"fmt"
	"net/http"
	//	"strings"

	"github.com/osbuild/osbuild-composer/internal/weldr"
)

// ListSourcesV0 returns a list of source names
func ListSourcesV0(socket *http.Client) ([]string, *APIResponse, error) {
	body, resp, err := GetRaw(socket, "GET", "/api/v0/projects/source/list")
	if resp != nil || err != nil {
		return nil, resp, err
	}
	var list weldr.SourceListV0
	err = json.Unmarshal(body, &list)
	if err != nil {
		return nil, nil, err
	}
	return list.Sources, nil, nil
}

// ListSourcesV1 returns a list of source ids
func ListSourcesV1(socket *http.Client) ([]string, *APIResponse, error) {
	body, resp, err := GetRaw(socket, "GET", "/api/v1/projects/source/list")
	if resp != nil || err != nil {
		return nil, resp, err
	}
	var list weldr.SourceListV1
	err = json.Unmarshal(body, &list)
	if err != nil {
		return nil, nil, err
	}
	return list.Sources, nil, nil
}

// GetSourceInfoV0 returns detailed information on the named sources
func GetSourceInfoV0(socket *http.Client, sourceNames string) (map[string]weldr.SourceConfigV0, *APIResponse, error) {
	body, resp, err := GetRaw(socket, "GET", "/api/v0/projects/source/info/"+sourceNames)
	if resp != nil || err != nil {
		return nil, resp, err
	}
	var info weldr.SourceInfoV0
	err = json.Unmarshal(body, &info)
	if err != nil {
		return nil, nil, err
	}
	return info.Sources, nil, nil
}

// PostJSONSourceV0 sends a JSON source string to the API
// and returns an APIResponse
func PostJSONSourceV0(socket *http.Client, source string) (*APIResponse, error) {
	body, resp, err := PostJSON(socket, "/api/v0/projects/source/new", source)
	if resp != nil || err != nil {
		return resp, err
	}
	return NewAPIResponse(body)
}

// PostTOMLSourceV0 sends a TOML source string to the API
// and returns an APIResponse
func PostTOMLSourceV0(socket *http.Client, source string) (*APIResponse, error) {
	body, resp, err := PostTOML(socket, "/api/v0/projects/source/new", source)
	if resp != nil || err != nil {
		return resp, err
	}
	return NewAPIResponse(body)
}

// DeleteSourceV0 deletes the named source and returns an APIResponse
func DeleteSourceV0(socket *http.Client, sourceName string) (*APIResponse, error) {
	body, resp, err := DeleteRaw(socket, "/api/v0/projects/source/delete/"+sourceName)
	if resp != nil || err != nil {
		return resp, err
	}
	return NewAPIResponse(body)
}
