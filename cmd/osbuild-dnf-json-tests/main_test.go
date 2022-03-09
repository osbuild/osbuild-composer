// This package contains tests related to dnf-json and rpmmd package.

// +build integration

package main

import (
	"net/http"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	fedora "github.com/osbuild/osbuild-composer/internal/distro/fedora33"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/test"
)

func TestFetchChecksum(t *testing.T) {
	dir, err := test.SetUpTemporaryRepository()
	fs := http.FileServer(http.Dir(dir))
	go func() {
		err := http.ListenAndServe(":9000", fs)
		assert.Nilf(t, err, "Could not start the http server: %v", err)
	}()
	defer func(dir string) {
		err := test.TearDownTemporaryRepository(dir)
		assert.Nil(t, err, "Failed to clean up temporary repository.")
	}(dir)
	assert.Nilf(t, err, "Failed to set up temporary repository: %v", err)

	repoCfg := rpmmd.RepoConfig{
		Name:      "repo",
		BaseURL:   "http://localhost:9000",
		IgnoreSSL: true,
	}

	// use a fullpath to dnf-json, this allows this test to have an arbitrary
	// working directory
	rpmMetadata := rpmmd.NewRPMMD(path.Join(dir, "rpmmd"))
	_, c, err := rpmMetadata.FetchMetadata([]rpmmd.RepoConfig{repoCfg}, "platform:f31", "x86_64", "31")
	assert.Nilf(t, err, "Failed to fetch checksum: %v", err)
	assert.NotEqual(t, "", c["repo"], "The checksum is empty")
}

// This test loads all the repositories available in /repositories directory
// and tries to run depsolve for each architecture. With N architectures available
// this should run cross-arch dependency solving N-1 times.
func TestCrossArchDepsolve(t *testing.T) {
	// Load repositories from the definition we provide in the RPM package
	repoDir := "/usr/share/tests/osbuild-composer"

	// NOTE: we can add RHEL, but don't make it hard requirement because it will fail outside of VPN
	for _, distroStruct := range []distro.Distro{fedora.NewF35()} {
		t.Run(distroStruct.Name(), func(t *testing.T) {

			// Run tests in parallel to speed up run times.
			t.Parallel()

			// Set up temporary directory for rpm/dnf cache
			dir := t.TempDir()

			// use a fullpath to dnf-json, this allows this test to have an arbitrary
			// working directory
			rpm := rpmmd.NewRPMMD(dir)

			repos, err := rpmmd.LoadRepositories([]string{repoDir}, distroStruct.Name())
			require.NoErrorf(t, err, "Failed to LoadRepositories %v", distroStruct.Name())

			for _, archStr := range distroStruct.ListArches() {
				t.Run(archStr, func(t *testing.T) {
					arch, err := distroStruct.GetArch(archStr)
					require.NoError(t, err)

					for _, imgTypeStr := range arch.ListImageTypes() {
						t.Run(imgTypeStr, func(t *testing.T) {
							imgType, err := arch.GetImageType(imgTypeStr)
							require.NoError(t, err)

							packages := imgType.PackageSets(blueprint.Blueprint{})

							_, _, err = rpm.Depsolve(packages["build-packages"], repos[archStr], distroStruct.ModulePlatformID(), archStr, distroStruct.Releasever())
							assert.NoError(t, err)

							_, _, err = rpm.Depsolve(packages["packages"], repos[archStr], distroStruct.ModulePlatformID(), archStr, distroStruct.Releasever())
							assert.NoError(t, err)
						})
					}
				})
			}
		})
	}
}
