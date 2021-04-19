// Package weldrcheck contains functions used to run integration tests on a running API server
// Copyright (C) 2020 by Red Hat, Inc.

// nolint: deadcode,unused // These functions are used by the *_test.go code
package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
)

type TestState struct {
	socket        *http.Client
	apiVersion    int
	repoDir       string
	unitTest      bool
	imageTypeName string
}

func setUpTestState(socketPath string, imageTypeName string, unitTest bool) (*TestState, error) {
	state := TestState{imageTypeName: imageTypeName, unitTest: unitTest}

	state.socket = &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	// Make sure the server is running
	status, resp, err := GetStatusV0(state.socket)
	if err != nil {
		return nil, fmt.Errorf("status request failed with client error: %s", err)
	}
	if resp != nil {
		return nil, fmt.Errorf("status request failed: %v\n", resp)
	}
	apiVersion, e := strconv.Atoi(status.API)
	if e != nil {
		state.apiVersion = 0
	} else {
		state.apiVersion = apiVersion
	}
	fmt.Printf("Running tests against %s %s server using V%d API\n\n", status.Backend, status.Build, state.apiVersion)

	return &state, nil
}
