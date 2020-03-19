// Package client - weldr contains functions to return API structures
// Copyright (C) 2020 by Red Hat, Inc.
package client

import (
	"encoding/json"
	"net/http"

	"github.com/osbuild/osbuild-composer/internal/weldr"
)

// GetStatusV0 makes a GET request to /api/status and returns the v0 response as a StatusResponseV0
func GetStatusV0(socket *http.Client) (reply weldr.StatusV0, resp *APIResponse, err error) {
	body, resp, err := GetRaw(socket, "GET", "/api/status")
	if resp != nil || err != nil {
		return reply, resp, err
	}
	err = json.Unmarshal(body, &reply)
	if err != nil {
		return reply, nil, err
	}
	return reply, nil, nil
}
