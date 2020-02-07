// Package client - weldr contains functions to return API structures
// Copyright (C) 2020 by Red Hat, Inc.
package client

import (
	"encoding/json"
)

// GetStatusV0 makes a GET request to /api/status and returns the v0 response as a StatusResponseV0
func GetStatusV0(socket string) (reply StatusV0, err *APIResponse) {
	body, err := GetRaw(socket, "GET", "/api/status")
	if err != nil {
		return reply, err
	}
	jerr := json.Unmarshal(body, &reply)
	if jerr != nil {
		return reply, clientError(jerr)
	}
	return reply, nil
}
