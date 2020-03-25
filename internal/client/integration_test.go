// Package client - integration_test contains functions to setup integration tests
// Copyright (C) 2020 by Red Hat, Inc.

// +build integration

package client

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/osbuild/osbuild-composer/internal/test"
)

// Hold test state to share between tests
var testState *TestState

// Setup the socket to use for running the tests
// Also makes sure there is a running server to test against
func TestMain(m *testing.M) {
	var err error
	testState, err = setUpTestState("/run/weldr/api.socket", 60*time.Second)
	if err != nil {
		fmt.Printf("ERROR: Test setup failed: %s\n", err)
		os.Exit(1)
	}

	// Setup the test repo
	dir, err := test.SetUpTemporaryRepository()
	if err != nil {
		fmt.Printf("ERROR: Test repo setup failed: %s\n", err)
		os.Exit(1)
	}
	testState.repoDir = dir

	// Run the tests
	rc := m.Run()

	// Cleanup after the tests
	err = test.TearDownTemporaryRepository(dir)
	if err != nil {
		fmt.Printf("ERROR: Failed to clean up temporary repository: %s\n", err)
	}
	os.Exit(rc)
}
