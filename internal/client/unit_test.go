// Package client contains functions for communicating with the API server
// Copyright (C) 2020 by Red Hat, Inc.

//go:build !integration

package client

import (
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"testing"

	"github.com/osbuild/images/pkg/distro/test_distro"
	"github.com/osbuild/images/pkg/dnfjson"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/rpmmd"
	depsolvednf_mock "github.com/osbuild/osbuild-composer/internal/mocks/depsolvednf"
	rpmmd_mock "github.com/osbuild/osbuild-composer/internal/mocks/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/weldr"
)

// Hold test state to share between tests
var testState *TestState

func executeTests(m *testing.M) int {
	// Setup the mocked server running on a temporary domain socket
	tmpdir, err := os.MkdirTemp("", "client_test-")
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
	fixture := rpmmd_mock.BaseFixture(path.Join(tmpdir, "/jobs"), test_distro.TestDistro1Name, test_distro.TestArchName)
	defer fixture.StoreFixture.Cleanup()

	_, err = fixture.Workers.RegisterWorker("", fixture.StoreFixture.HostArchName)
	if err != nil {
		panic(err)
	}

	rr := reporegistry.NewFromDistrosRepoConfigs(rpmmd.DistrosRepoConfigs{
		fixture.StoreFixture.HostDistroName: {
			fixture.StoreFixture.HostArchName: {
				{Name: "test-system-repo", BaseURLs: []string{"http://example.com/test/os/test_arch"}},
			},
		},
	})

	dspath, err := os.MkdirTemp(tmpdir, "")
	if err != nil {
		log.Fatalf("ERROR: Could not create temporary directory for dnfjson: %s\n", err)
	}
	defer os.RemoveAll(dspath)

	logger := log.New(os.Stdout, "", 0)

	getSolverFn := func(modulePlatformID, releaseVer, arch, distro string) weldr.Solver {
		return &depsolvednf_mock.MockDepsolveDNF{
			DepsolveRes: &dnfjson.DepsolveResult{
				Packages: depsolvednf_mock.BaseDepsolveResult(""),
			},
			FetchRes:     depsolvednf_mock.BaseFetchResult(),
			SearchResMap: depsolvednf_mock.BaseSearchResultsMap(),
		}
	}

	api := weldr.NewTestAPI(getSolverFn, rr, logger, fixture.StoreFixture, fixture.Workers, "", nil)
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
