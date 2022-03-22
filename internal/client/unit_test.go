// Package client contains functions for communicating with the API server
// Copyright (C) 2020 by Red Hat, Inc.

//go:build !integration
// +build !integration

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

	"github.com/osbuild/osbuild-composer/internal/distro/test_distro"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/dnfjson"
	dnfjson_mock "github.com/osbuild/osbuild-composer/internal/mocks/dnfjson"
	rpmmd_mock "github.com/osbuild/osbuild-composer/internal/mocks/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/reporegistry"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/weldr"
)

// Hold test state to share between tests
var testState *TestState
var dnfjsonPath string

func init() {
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
	fixture := rpmmd_mock.BaseFixture(path.Join(tmpdir, "/jobs"))

	distro1 := test_distro.New()
	arch, err := distro1.GetArch(test_distro.TestArchName)
	if err != nil {
		panic(err)
	}
	distro2 := test_distro.New2()

	rr := reporegistry.NewFromDistrosRepoConfigs(rpmmd.DistrosRepoConfigs{
		test_distro.TestDistroName: {
			test_distro.TestArchName: {
				{Name: "test-system-repo", BaseURL: "http://example.com/test/os/test_arch"},
			},
		},
	})

	dr, err := distroregistry.New(distro1, distro1, distro2)
	if err != nil {
		panic(err)
	}

	dspath, err := os.MkdirTemp(tmpdir, "")
	dnfjsonFixture := dnfjson_mock.Base(dspath)
	solver := dnfjson.NewBaseSolver(path.Join(tmpdir, "dnfjson-cache"))
	solver.SetDNFJSONPath(dnfjsonPath, dnfjsonFixture)
	logger := log.New(os.Stdout, "", 0)
	api := weldr.NewTestAPI(solver, arch, dr, rr, logger, fixture.Store, fixture.Workers, "", nil)
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
