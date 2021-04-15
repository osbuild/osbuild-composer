// Package client contains functions for communicating with the API server
// Copyright (C) 2020 by Red Hat, Inc.

// +build !integration

package client

import (
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/distro/test_distro"
	rpmmd_mock "github.com/osbuild/osbuild-composer/internal/mocks/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/reporegistry"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/weldr"
)

// Hold test state to share between tests
var testState *TestState

func executeTests(m *testing.M) int {
	// Setup the mocked server running on a temporary domain socket
	tmpdir, err := ioutil.TempDir("", "client_test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpdir)

	socketPath := tmpdir + "/client_test.socket"
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		panic(err)
	}

	// Create a mock API server listening on the temporary socket
	err = os.Mkdir(path.Join(tmpdir, "/jobs"), 0755)
	if err != nil {
		panic(err)
	}
	fixture := rpmmd_mock.BaseFixture(path.Join(tmpdir, "/jobs"))
	rpm := rpmmd_mock.NewRPMMDMock(fixture)
	distro := test_distro.New()
	arch, err := distro.GetArch(test_distro.TestArchName)
	if err != nil {
		panic(err)
	}

	rr := reporegistry.NewFromDistrosRepoConfigs(rpmmd.DistrosRepoConfigs{
		test_distro.TestDistroName: {
			test_distro.TestArchName: {
				{Name: "test-system-repo", BaseURL: "http://example.com/test/os/test_arch"},
			},
		},
	})

	logger := log.New(os.Stdout, "", 0)
	api := weldr.NewTestAPI(rpm, arch, distro, rr, logger, fixture.Store, fixture.Workers, "")
	server := http.Server{Handler: api}
	defer server.Close()

	go func() {
		err := server.Serve(ln)
		if err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	testState, err = setUpTestState(socketPath, test_distro.TestImageTypeName, true)
	if err != nil {
		log.Fatalf("ERROR: Test setup failed: %s\n", err)
	}

	// Run the tests
	return m.Run()
}

func TestMain(m *testing.M) {
	os.Exit(executeTests(m))
}
