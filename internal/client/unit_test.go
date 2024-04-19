// Package client contains functions for communicating with the API server
// Copyright (C) 2020 by Red Hat, Inc.

//go:build !integration

package client

import (
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"

	"github.com/osbuild/images/pkg/distro/test_distro"
	"github.com/osbuild/images/pkg/dnfjson"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/rpmmd"
	dnfjson_mock "github.com/osbuild/osbuild-composer/internal/mocks/dnfjson"
	rpmmd_mock "github.com/osbuild/osbuild-composer/internal/mocks/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/weldr"
)

// Hold test state to share between tests
var testState *TestState
var dnfjsonPath string

func setupDNFJSON() string {
	// compile the mock-dnf-json binary to speed up tests
	tmpdir, err := os.MkdirTemp("", "")
	if err != nil {
		panic(err)
	}
	dnfjsonPath = filepath.Join(tmpdir, "mock-dnf-json")
	cmd := exec.Command("go", "build", "-o", dnfjsonPath, "../../cmd/mock-dnf-json")
	if err := cmd.Run(); err != nil {
		panic(err)
	}
	return tmpdir
}

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
	dnfjsonFixture := dnfjson_mock.Base(dspath)
	solver := dnfjson.NewBaseSolver(path.Join(tmpdir, "dnfjson-cache"))
	solver.SetDNFJSONPath(dnfjsonPath, dnfjsonFixture)
	logger := log.New(os.Stdout, "", 0)
	api := weldr.NewTestAPI(solver, rr, logger, fixture.StoreFixture, fixture.Workers, "", nil)
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
	tmpdir := setupDNFJSON()
	defer os.RemoveAll(tmpdir)
	os.Exit(executeTests(m))
}
