// Package weldrcheck contains functions used to run integration tests on a running API server
// Copyright (C) 2020 by Red Hat, Inc.

// +build integration

// nolint: deadcode // These functions are used by the *_test.go code
package weldrcheck

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/osbuild/osbuild-composer/internal/client"
)

type TestState struct {
	socket     *http.Client
	apiVersion int
	repoDir    string
}

// isStringInSlice returns true if the string is present, false if not
// slice must be sorted
// TODO decide if this belongs in a more widely useful package location
func isStringInSlice(slice []string, s string) bool {
	i := sort.SearchStrings(slice, s)
	if i < len(slice) && slice[i] == s {
		return true
	}
	return false
}

func setUpTestState(socketPath string, timeout time.Duration) (*TestState, error) {
	var state TestState
	state.socket = &http.Client{
		// TODO This may be too short/simple for downloading images
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	// Make sure the server is running
	status, resp, err := client.GetStatusV0(state.socket)
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
