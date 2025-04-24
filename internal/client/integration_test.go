// Package client - integration_test contains functions to setup integration tests
// Copyright (C) 2020 by Red Hat, Inc.

//go:build integration

package client

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/osbuild-composer/internal/test"
)

// Hold test state to share between tests
var testState *TestState

// Setup the socket to use for running the tests
// Also makes sure there is a running server to test against
func executeTests(m *testing.M) int {
	var err error

	imageType := "qcow2"

	// Fedora now only defines qcow2 as an alias, which break some checks
	// (e.g. aliases aren't returned when listing image types), so we need to
	// use the real image type name.
	distroName, err := distro.GetHostDistroName()
	if err != nil {
		fmt.Printf("ERROR: Failed to get distro name: %s\n", err)
		panic(err)
	}
	if strings.HasPrefix(distroName, "fedora") {
		imageType = "server-qcow2"
	}

	testState, err = setUpTestState("/run/weldr/api.socket", imageType, false)
	if err != nil {
		fmt.Printf("ERROR: Test setup failed: %s\n", err)
		panic(err)
	}

	// Setup the test repo
	dir, err := test.SetUpTemporaryRepository()
	if err != nil {
		fmt.Printf("ERROR: Test repo setup failed: %s\n", err)
		panic(err)
	}

	// Cleanup after the tests
	defer func() {
		err := test.TearDownTemporaryRepository(dir)
		if err != nil {
			fmt.Printf("ERROR: Failed to clean up temporary repository: %s\n", err)
		}
	}()

	testState.repoDir = dir

	// Run the tests
	return m.Run()
}

func TestMain(m *testing.M) {
	os.Exit(executeTests(m))
}
